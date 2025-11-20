package app

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"gorm.io/gorm/schema"

	"github.com/example/user-service/config"
	"github.com/example/user-service/internal/ports/broker"
	"github.com/example/user-service/internal/ports/filestorage"
	httpport "github.com/example/user-service/internal/ports/http"
	"github.com/example/user-service/internal/ports/http/handlers"
	mw "github.com/example/user-service/internal/ports/http/middleware"
	rbacclient "github.com/example/user-service/internal/ports/rbac"
	"github.com/example/user-service/internal/ports/tarantool"
	"github.com/example/user-service/internal/repo"
	"github.com/example/user-service/internal/service"
	pkglog "github.com/example/user-service/pkg/log"
)

type App struct {
	cfg       *config.Config
	logger    pkglog.Logger
	db        *gorm.DB
	publisher broker.Publisher
	echo      *echo.Echo
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

	publisher, err := broker.NewRabbitMQPublisher(cfg.RabbitMQURL, cfg.RabbitMQExchange)
	if err != nil {
		log.Printf("rabbitmq init failed: %v", err)
	}

	userRepo := repo.NewUserRepository(db)
	profileRepo := repo.NewUserProfileRepository(db)
	identityRepo := repo.NewUserIdentityRepository(db)
	signer, err := service.NewJWTSigner(cfg)
	if err != nil {
		return nil, err
	}
	avatarIngestor := service.NewAvatarIngestor(filestorageClient, logger)
	authService := service.NewAuthService(cfg, logger, userRepo, profileRepo, tarantoolClient, publisher, signer, avatarIngestor)
	userService := service.NewUserService(userRepo, profileRepo, tarantoolClient)

	authHandler := handlers.NewAuthHandler(authService)
	userHandler := handlers.NewUserHandler(userService)

	authMW := mw.NewAuthMiddleware(cfg, logger, rbacClient)
	rbacMW := mw.NewRBACMiddleware(rbacClient)

	e := echo.New()
	router := httpport.NewRouter(cfg, authHandler, userHandler, authMW, rbacMW)
	router.Setup(e)

	return &App{cfg: cfg, logger: logger, db: db, publisher: publisher, echo: e}, nil
}

func (a *App) Run(ctx context.Context) error {
	server := &http.Server{
		Addr:    ":" + a.cfg.AppPort,
		Handler: a.echo,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = a.echo.Shutdown(shutdownCtx)
	}()
	return server.ListenAndServe()
}

func (a *App) Close() {
	if a.publisher != nil {
		_ = a.publisher.Close()
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
