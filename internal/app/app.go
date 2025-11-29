package app

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/nats-io/nats.go"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/example/user-service/config"
	"github.com/example/user-service/internal/adapter/broker"
	"github.com/example/user-service/internal/adapter/filestorage"
	httpport "github.com/example/user-service/internal/adapter/http"
	"github.com/example/user-service/internal/adapter/http/handlers"
	mw "github.com/example/user-service/internal/adapter/http/middleware"
	"github.com/example/user-service/internal/adapter/imageprocessor"
	natsadapter "github.com/example/user-service/internal/adapter/nats"
	"github.com/example/user-service/internal/adapter/postgres"
	rbacclient "github.com/example/user-service/internal/adapter/rbac"
	"github.com/example/user-service/internal/adapter/tarantool"
	"github.com/example/user-service/internal/usecase"
	pkglog "github.com/example/user-service/pkg/log"
)

type App struct {
	cfg       *config.Config
	logger    pkglog.Logger
	db        *gorm.DB
	publisher broker.Publisher
	echo      *echo.Echo
	natsConn  *nats.Conn
}

func New(ctx context.Context) (*App, error) {
	cfg := config.MustLoad()
	logger := pkglog.New(cfg.AppEnv)

	db, err := gorm.Open(postgres.Open(buildDSN(cfg)), &gorm.Config{
		Logger: loggerForGorm(cfg),
		NamingStrategy: schema.NamingStrategy{
			SingularTable: true,
		},
	})
	if err != nil {
		return nil, err
	}

	tarantoolClient := tarantool.NewHTTPClient(cfg.TarantoolURL, 5*time.Second)
	filestorageClient := filestorage.NewHTTPClient(cfg.FileStorageURL, 5*time.Second)
	rbacHTTP := rbacclient.NewHTTPClient(cfg.RBACURL, 3*time.Second)
	rbacClient := rbacclient.NewCachingClient(rbacHTTP, time.Minute)

	var publisher broker.Publisher
	var natsConn *nats.Conn
	switch cfg.MessageBroker {
	case "nats":
		publisher, err = broker.NewNATSPublisher(cfg.NATSURL)
		if err != nil {
			log.Printf("nats init failed: %v", err)
		}
		natsConn, _ = nats.Connect(cfg.NATSURL)
	default:
		publisher, err = broker.NewRabbitMQPublisher(cfg.RabbitMQURL, cfg.RabbitMQExchange)
		if err != nil {
			log.Printf("rabbitmq init failed: %v", err)
		}
	}
	if natsConn == nil && cfg.NATSURL != "" {
		// Connect to NATS for RBAC role checks even when RabbitMQ is used as the primary broker.
		if conn, err := nats.Connect(cfg.NATSURL); err == nil {
			natsConn = conn
		} else {
			log.Printf("nats connection for rbac failed: %v", err)
		}
	}

	userRepo := repo.NewUserRepository(db)
	profileRepo := repo.NewUserProfileRepository(db)
	providerRepo := repo.NewUserProviderRepository(db)
	identityRepo := repo.NewUserIdentityRepository(db)
	userService := service.NewUserService(userRepo, profileRepo, identityRepo, tarantoolClient)
	manageService := service.NewUserManageService(userRepo, profileRepo, rbacClient)

	var imageProcClient imageprocessor.Client
	if cfg.ImageProcessorURL != "" {
		imageProcClient = imageprocessor.NewHTTPClient(cfg.ImageProcessorURL, 10*time.Second)
	}

	userHandler := handlers.NewUserHandler(userService, filestorageClient, imageProcClient, cfg.AvatarPresetGroup, cfg.AvatarFileKind)
	manageHandler := handlers.NewUserManageHandler(manageService)

	authMW := mw.NewAuthMiddleware(cfg, logger, rbacClient, userRepo, natsConn)
	rbacMW := mw.NewRBACMiddleware(rbacClient)

	e := echo.New()
	router := httpport.NewRouter(cfg, userHandler, manageHandler, authMW, rbacMW)
	router.Setup(e)

	if natsConn != nil {
		rpc := natsadapter.Server{Conn: natsConn}
		createHandler := natsadapter.NewCreateUserHandler(userRepo, profileRepo)
		_ = rpc.Subscribe(cfg.NATSUserCreate, "ms-go-user", createHandler.Handle)
	}

	return &App{cfg: cfg, logger: logger, db: db, publisher: publisher, echo: e, natsConn: natsConn}, nil
}

func (a *App) Run(ctx context.Context) error {
	errCh := make(chan error, 1)
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.echo.Shutdown(shutdownCtx)
	}()
	go func() {
		errCh <- a.echo.Start(":" + a.cfg.AppPort)
	}()
	select {
	case <-ctx.Done():
		return nil
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	}
}

func (a *App) Close() {
	if a.publisher != nil {
		_ = a.publisher.Close()
	}
	if a.natsConn != nil {
		_ = a.natsConn.Drain()
	}
	if a.db != nil {
		if sqlDB, err := a.db.DB(); err == nil {
			_ = sqlDB.Close()
		}
	}
}

func buildDSN(cfg *config.Config) string {
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", cfg.DBHost, cfg.DBPort, cfg.DBUser, cfg.DBPassword, cfg.DBName, cfg.DBSSLMode)
}

func loggerForGorm(cfg *config.Config) logger.Interface {
	level := logger.Silent
	switch cfg.GormLogLevel {
	case "error":
		level = logger.Error
	case "warn":
		level = logger.Warn
	case "info":
		level = logger.Info
	}
	return logger.Default.LogMode(level)
}
