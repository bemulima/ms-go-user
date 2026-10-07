// Package config loads environment-backed settings for the user service.
package config

import (
	"log"
	"time"

	"github.com/caarlos0/env/v10"
	"github.com/joho/godotenv"
)

// Config contains the user service's network, persistence, and integration settings.
type Config struct {
	AppName          string `env:"APP_NAME" envDefault:"user-service"`
	AppEnv           string `env:"APP_ENV" envDefault:"local"`
	AppHost          string `env:"APP_HOST"`
	AppPort          string `env:"APP_PORT" envDefault:"8080"`
	AppPublicURL     string `env:"APP_PUBLIC_URL" envDefault:"http://localhost:8000"`
	DBHost           string `env:"DB_HOST" envDefault:"localhost"`
	DBPort           string `env:"DB_PORT" envDefault:"5432"`
	DBUser           string `env:"DB_USER" envDefault:"app"`
	DBPassword       string `env:"DB_PASSWORD" envDefault:"app_password"`
	DBName           string `env:"DB_NAME" envDefault:"userdb"`
	DBSSLMode        string `env:"DB_SSLMODE" envDefault:"disable"`
	GormLogLevel     string `env:"GORM_LOG_LEVEL" envDefault:"warn"`
	DBMigrateOnStart bool   `env:"DB_MIGRATE_ON_START" envDefault:"true"`

	JWTSecret            string        `env:"JWT_SECRET"`
	JWTPrivateKey        string        `env:"JWT_PRIVATE_KEY"`
	JWTPublicKey         string        `env:"JWT_PUBLIC_KEY"`
	JWTTTLMinutes        time.Duration `env:"JWT_TTL_MINUTES" envDefault:"60m"`
	JWTRefreshTTLMinutes time.Duration `env:"JWT_REFRESH_TTL_MINUTES" envDefault:"43200m"`
	JWTIssuer            string        `env:"JWT_ISSUER" envDefault:"user-service"`
	JWTAudience          string        `env:"JWT_AUDIENCE" envDefault:"frontend"`
	NATSAuthVerify       string        `env:"NATS_SUBJECT_AUTH_VERIFY" envDefault:"auth.verifyJWT"`

	GoogleClientID     string `env:"GOOGLE_CLIENT_ID"`
	GoogleClientSecret string `env:"GOOGLE_CLIENT_SECRET"`
	GoogleRedirectURL  string `env:"GOOGLE_REDIRECT_URL"`

	GithubClientID     string `env:"GITHUB_CLIENT_ID"`
	GithubClientSecret string `env:"GITHUB_CLIENT_SECRET"`
	GithubRedirectURL  string `env:"GITHUB_REDIRECT_URL"`

	FileStorageURL              string `env:"MS_FILESTORAGE_URL" envDefault:"http://ms-filestorage:8000"`
	FileStorageInternalToken    string `env:"FILESTORAGE_INTERNAL_TOKEN"`
	ImageProcessorURL           string `env:"IMAGE_PROCESSOR_SERVICE_BASE_URL"`
	ImageProcessorInternalToken string `env:"IMAGE_PROCESSOR_INTERNAL_TOKEN"`
	AvatarPresetGroup           string `env:"AVATAR_PRESET_GROUP" envDefault:"avatar"`
	AvatarFileKind              string `env:"AVATAR_FILE_KIND" envDefault:"USER_MEDIA"`

	RBACURL string `env:"MS_RBAC"`

	NATSURL          string `env:"NATS_URL" envDefault:"nats://localhost:4222"`
	NATSUserCreate   string `env:"NATS_SUBJECT_USER_CREATE" envDefault:"user.create-user"`
	InternalAPIToken string `env:"INTERNAL_API_TOKEN" envDefault:"change-me"`

	CORSAllowOrigins string `env:"CORS_ALLOW_ORIGINS" envDefault:"*"`
	RateLimitPerMin  int    `env:"RATE_LIMIT_PER_MIN" envDefault:"120"`
}

// Load reads optional dotenv values and parses the process environment.
func Load() (*Config, error) {
	_ = godotenv.Load()
	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

// MustLoad loads configuration and terminates the process if parsing fails.
func MustLoad() *Config {
	cfg, err := Load()
	if err != nil {
		log.Fatalf("failed to load config: %v", err)
	}
	return cfg
}
