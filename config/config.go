package config

import (
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type TheMonkeysGateway struct {
	HTTPS        string `mapstructure:"HTTPS"`
	HTTP         string `mapstructure:"HTTP"`
	HTTPPort     int    `mapstructure:"http_port"`
	InternalPort int    `mapstructure:"internal_port"`
}
type Microservices struct {
	TheMonkeysAuthz            string `mapstructure:"the_monkeys_authz"`
	AuthzPort                  int    `mapstructure:"authz_port"`
	AuthzInternalPort          int    `mapstructure:"authz_internal_port"`
	TheMonkeysBlog             string `mapstructure:"the_monkeys_blog"`
	BlogPort                   int    `mapstructure:"blog_port"`
	BlogInternalPort           int    `mapstructure:"blog_internal_port"`
	TheMonkeysUser             string `mapstructure:"the_monkeys_user"`
	UserPort                   int    `mapstructure:"user_port"`
	UserInternalPort           int    `mapstructure:"user_internal_port"`
	TheMonkeysFileStore        string `mapstructure:"the_monkeys_storage"`
	StoragePort                int    `mapstructure:"storage_port"`
	StorageInternalPort        int    `mapstructure:"storage_internal_port"`
	TheMonkeysNotification     string `mapstructure:"the_monkeys_notification"`
	NotificationPort           int    `mapstructure:"notification_port"`
	NotificationInternalPort   int    `mapstructure:"notification_internal_port"`
	TheMonkeysCache            string `mapstructure:"the_monkeys_cache"`
	TheMonkeysAIEngine         string `mapstructure:"the_monkeys_ai_engine"`
	AIEnginePort               int    `mapstructure:"ai_engine_port"`
	AIEngineInternalPort       int    `mapstructure:"ai_engine_internal_port"`
	AIEngineHealthPort         int    `mapstructure:"ai_engine_health_port"`
	AIEngineHealthInternalPort int    `mapstructure:"ai_engine_health_internal_port"`
}

type Database struct {
	DBUsername   string `mapstructure:"db_username"`
	DBPassword   string `mapstructure:"db_password"`
	DBHost       string `mapstructure:"db_host"`
	DBPort       int    `mapstructure:"db_port"`
	InternalPort int    `mapstructure:"internal_port"`
	DBName       string `mapstructure:"db_name"`
}

type Postgresql struct {
	PrimaryDB Database `mapstructure:"primary_db"`
	Replica1  Database `mapstructure:"replica_1"`
}

type JWT struct {
	SecretKey string `mapstructure:"secret_key"`
}

type Opensearch struct {
	Address       string `mapstructure:"address"`
	Host          string `mapstructure:"os_host"`
	Username      string `mapstructure:"os_username"`
	Password      string `mapstructure:"os_password"`
	HttpPort      int    `mapstructure:"http_port"`
	TransportPort int    `mapstructure:"transport_port"`
}

type Email struct {
	SMTPAddress  string `mapstructure:"smtp_address"`
	SMTPMail     string `mapstructure:"smtp_mail"`
	SMTPPassword string `mapstructure:"smtp_password"`
	SMTPHost     string `mapstructure:"smtp_host"`
}

type Gmail struct {
	SMTPAddress  string `mapstructure:"smtp_address"`
	SMTPMail     string `mapstructure:"smtp_mail"`
	SMTPPassword string `mapstructure:"smtp_password"`
	SMTPHost     string `mapstructure:"smtp_host"`
}

type Authentication struct {
	EmailVerificationAddr string `mapstructure:"email_verification_addr"`
}

type RabbitMQ struct {
	Protocol               string   `mapstructure:"protocol"`
	Host                   string   `mapstructure:"host"`
	Port                   string   `mapstructure:"port"`
	InternalPort           string   `mapstructure:"internal_port"`
	ManagementPort         string   `mapstructure:"managementport"`
	ManagementInternalPort string   `mapstructure:"management_internal_port"`
	Username               string   `mapstructure:"username"`
	Password               string   `mapstructure:"password"`
	VirtualHost            string   `mapstructure:"virtual_host"`
	Exchange               string   `mapstructure:"exchange"`
	Queues                 []string `yaml:"queues"`
	RoutingKeys            []string `yaml:"routingKeys"`
}

type Keys struct {
	MediaStack     string `mapstructure:"mediastack"`
	NewsApi        string `mapstructure:"newsapi"`
	HindustanTimes string `mapstructure:"hindustantimes"`
	GitHubToken    string `mapstructure:"github_token"`
	AdminSecretKey string `mapstructure:"admin_secret_key"`
	SystemKey      string `mapstructure:"system_key"` // Key for system-level operations
}

type GoogleOAuth2 struct {
	RedirectURL  string   `mapstructure:"redirect_url"`
	ClientID     string   `mapstructure:"client_id"`
	ClientSecret string   `mapstructure:"client_secret"`
	Scope        []string `mapstructure:"scope"`
	Endpoint     string   `mapstructure:"endpoint"`
}

type Cors struct {
	AllowedOriginExp string `mapstructure:"allowed_origin_regexp"`
	UseTempCors      bool   `mapstructure:"use_temp_cors"`
}

type Redis struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
	PoolSize int    `mapstructure:"pool_size"`
	MaxIdle  int    `mapstructure:"max_idle"`
}

type Config struct {
	TheMonkeysGateway TheMonkeysGateway `mapstructure:"the_monkeys_gateway"`
	Microservices     Microservices     `mapstructure:"microservices"`
	Postgresql        Postgresql        `mapstructure:"postgresql"`
	JWT               JWT               `mapstructure:"jwt"`
	Opensearch        Opensearch        `mapstructure:"opensearch"`
	Email             Email             `mapstructure:"email"`
	Gmail             Gmail             `mapstructure:"gmail"`
	Authentication    Authentication    `mapstructure:"authentication"`
	RabbitMQ          RabbitMQ          `mapstructure:"rabbitMQ"`
	Keys              Keys              `mapstructure:"keys"`
	GoogleOAuth2      GoogleOAuth2      `mapstructure:"google_oauth2"`
	Cors              Cors              `mapstructure:"cors"`
	Redis             Redis             `mapstructure:"redis"`
}

func GetConfig() (*Config, error) {
	// Load .env file if it exists
	if err := godotenv.Load(".env"); err != nil {
		logrus.Warnf("No .env file found or error loading .env file: %v", err)
	} else {
		logrus.Info("Successfully loaded .env file")
	}

	// Configure Viper for environment variables
	viper.SetEnvPrefix("") // No prefix for env vars
	viper.AutomaticEnv()   // Ensures environment variables are loaded

	// Set environment variable key replacer to handle nested structs
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Bind environment variables to config keys
	bindEnvVars()

	// Comment out YAML reading to test environment variables only
	// viper.SetConfigName("config")
	// viper.SetConfigType("yaml") // Ensure this is yaml
	// viper.AddConfigPath("./config")

	config := &Config{}

	// Comment out YAML config reading for testing
	// Binding struct to config
	// if err := viper.ReadInConfig(); err != nil {
	// 	logrus.Errorf("Error reading config file, %v", err)
	// 	return config, err
	// }

	if err := viper.Unmarshal(config); err != nil {
		logrus.Errorf("Unable to decode into struct, %v", err)
		return config, err
	}

	// Handle array environment variables manually
	handleArrayEnvVars(config)

	logrus.Info("Configuration loaded from environment variables only")
	return config, nil
}

// bindEnvVars manually binds environment variables to viper keys
func bindEnvVars() {
	// Gateway
	viper.BindEnv("the_monkeys_gateway.HTTPS", "THE_MONKEYS_GATEWAY_HTTPS")
	viper.BindEnv("the_monkeys_gateway.HTTP", "THE_MONKEYS_GATEWAY_HTTP")
	viper.BindEnv("the_monkeys_gateway.http_port", "THE_MONKEYS_GATEWAY_HTTP_PORT")
	viper.BindEnv("the_monkeys_gateway.internal_port", "THE_MONKEYS_GATEWAY_INTERNAL_PORT")

	// Microservices
	viper.BindEnv("microservices.the_monkeys_authz", "MICROSERVICES_THE_MONKEYS_AUTHZ")
	viper.BindEnv("microservices.authz_port", "MICROSERVICES_AUTHZ_PORT")
	viper.BindEnv("microservices.authz_internal_port", "MICROSERVICES_AUTHZ_INTERNAL_PORT")
	viper.BindEnv("microservices.the_monkeys_blog", "MICROSERVICES_THE_MONKEYS_BLOG")
	viper.BindEnv("microservices.blog_port", "MICROSERVICES_BLOG_PORT")
	viper.BindEnv("microservices.blog_internal_port", "MICROSERVICES_BLOG_INTERNAL_PORT")
	viper.BindEnv("microservices.the_monkeys_user", "MICROSERVICES_THE_MONKEYS_USER")
	viper.BindEnv("microservices.user_port", "MICROSERVICES_USER_PORT")
	viper.BindEnv("microservices.user_internal_port", "MICROSERVICES_USER_INTERNAL_PORT")
	viper.BindEnv("microservices.the_monkeys_storage", "MICROSERVICES_THE_MONKEYS_STORAGE")
	viper.BindEnv("microservices.storage_port", "MICROSERVICES_STORAGE_PORT")
	viper.BindEnv("microservices.storage_internal_port", "MICROSERVICES_STORAGE_INTERNAL_PORT")
	viper.BindEnv("microservices.the_monkeys_notification", "MICROSERVICES_THE_MONKEYS_NOTIFICATION")
	viper.BindEnv("microservices.notification_port", "MICROSERVICES_NOTIFICATION_PORT")
	viper.BindEnv("microservices.notification_internal_port", "MICROSERVICES_NOTIFICATION_INTERNAL_PORT")
	viper.BindEnv("microservices.the_monkeys_cache", "MICROSERVICES_THE_MONKEYS_CACHE")
	viper.BindEnv("microservices.the_monkeys_ai_engine", "MICROSERVICES_THE_MONKEYS_AI_ENGINE")
	viper.BindEnv("microservices.ai_engine_port", "MICROSERVICES_AI_ENGINE_PORT")
	viper.BindEnv("microservices.ai_engine_internal_port", "MICROSERVICES_AI_ENGINE_INTERNAL_PORT")
	viper.BindEnv("microservices.ai_engine_health_port", "MICROSERVICES_AI_ENGINE_HEALTH_PORT")
	viper.BindEnv("microservices.ai_engine_health_internal_port", "MICROSERVICES_AI_ENGINE_HEALTH_INTERNAL_PORT")

	// PostgreSQL
	viper.BindEnv("postgresql.primary_db.db_username", "POSTGRESQL_PRIMARY_DB_DB_USERNAME")
	viper.BindEnv("postgresql.primary_db.db_password", "POSTGRESQL_PRIMARY_DB_DB_PASSWORD")
	viper.BindEnv("postgresql.primary_db.db_host", "POSTGRESQL_PRIMARY_DB_DB_HOST")
	viper.BindEnv("postgresql.primary_db.db_port", "POSTGRESQL_PRIMARY_DB_DB_PORT")
	viper.BindEnv("postgresql.primary_db.internal_port", "POSTGRESQL_PRIMARY_DB_INTERNAL_PORT")
	viper.BindEnv("postgresql.primary_db.db_name", "POSTGRESQL_PRIMARY_DB_DB_NAME")

	viper.BindEnv("postgresql.replica_1.db_username", "POSTGRESQL_REPLICA_1_DB_USERNAME")
	viper.BindEnv("postgresql.replica_1.db_password", "POSTGRESQL_REPLICA_1_DB_PASSWORD")
	viper.BindEnv("postgresql.replica_1.db_host", "POSTGRESQL_REPLICA_1_DB_HOST")
	viper.BindEnv("postgresql.replica_1.db_port", "POSTGRESQL_REPLICA_1_DB_PORT")
	viper.BindEnv("postgresql.replica_1.db_name", "POSTGRESQL_REPLICA_1_DB_NAME")

	// JWT
	viper.BindEnv("jwt.secret_key", "JWT_SECRET_KEY")

	// OpenSearch
	viper.BindEnv("opensearch.address", "OPENSEARCH_ADDRESS")
	viper.BindEnv("opensearch.os_host", "OPENSEARCH_OS_HOST")
	viper.BindEnv("opensearch.os_username", "OPENSEARCH_OS_USERNAME")
	viper.BindEnv("opensearch.os_password", "OPENSEARCH_OS_PASSWORD")
	viper.BindEnv("opensearch.http_port", "OPENSEARCH_HTTP_PORT")
	viper.BindEnv("opensearch.transport_port", "OPENSEARCH_TRANSPORT_PORT")

	// Email
	viper.BindEnv("email.smtp_address", "EMAIL_SMTP_ADDRESS")
	viper.BindEnv("email.smtp_mail", "EMAIL_SMTP_MAIL")
	viper.BindEnv("email.smtp_password", "EMAIL_SMTP_PASSWORD")
	viper.BindEnv("email.smtp_host", "EMAIL_SMTP_HOST")

	// Gmail
	viper.BindEnv("gmail.smtp_address", "GMAIL_SMTP_ADDRESS")
	viper.BindEnv("gmail.smtp_mail", "GMAIL_SMTP_MAIL")
	viper.BindEnv("gmail.smtp_password", "GMAIL_SMTP_PASSWORD")
	viper.BindEnv("gmail.smtp_host", "GMAIL_SMTP_HOST")

	// Authentication
	viper.BindEnv("authentication.email_verification_addr", "AUTHENTICATION_EMAIL_VERIFICATION_ADDR")

	// Google OAuth2
	viper.BindEnv("google_oauth2.redirect_url", "GOOGLE_OAUTH2_REDIRECT_URL")
	viper.BindEnv("google_oauth2.client_id", "GOOGLE_OAUTH2_CLIENT_ID")
	viper.BindEnv("google_oauth2.client_secret", "GOOGLE_OAUTH2_CLIENT_SECRET")
	viper.BindEnv("google_oauth2.endpoint", "GOOGLE_OAUTH2_ENDPOINT")

	// RabbitMQ
	viper.BindEnv("rabbitMQ.protocol", "RABBITMQ_PROTOCOL")
	viper.BindEnv("rabbitMQ.host", "RABBITMQ_HOST")
	viper.BindEnv("rabbitMQ.port", "RABBITMQ_PORT")
	viper.BindEnv("rabbitMQ.internal_port", "RABBITMQ_INTERNAL_PORT")
	viper.BindEnv("rabbitMQ.managementport", "RABBITMQ_MANAGEMENT_PORT")
	viper.BindEnv("rabbitMQ.management_internal_port", "RABBITMQ_MANAGEMENT_INTERNAL_PORT")
	viper.BindEnv("rabbitMQ.username", "RABBITMQ_USERNAME")
	viper.BindEnv("rabbitMQ.password", "RABBITMQ_PASSWORD")
	viper.BindEnv("rabbitMQ.virtual_host", "RABBITMQ_VIRTUAL_HOST")
	viper.BindEnv("rabbitMQ.exchange", "RABBITMQ_EXCHANGE")

	// Keys
	viper.BindEnv("keys.mediastack", "KEYS_MEDIASTACK")
	viper.BindEnv("keys.newsapi", "KEYS_NEWSAPI")
	viper.BindEnv("keys.hindustantimes", "KEYS_HINDUSTANTIMES")
	viper.BindEnv("keys.github_token", "KEYS_GITHUB_TOKEN")
	viper.BindEnv("keys.admin_secret_key", "KEYS_ADMIN_SECRET_KEY")
	viper.BindEnv("keys.system_key", "KEYS_SYSTEM_KEY")

	// CORS
	viper.BindEnv("cors.allowed_origin_regexp", "CORS_ALLOWED_ORIGIN_REGEXP")
	viper.BindEnv("cors.use_temp_cors", "CORS_USE_TEMP_CORS")

	// Redis
	viper.BindEnv("redis.host", "REDIS_HOST")
	viper.BindEnv("redis.port", "REDIS_PORT")
	viper.BindEnv("redis.password", "REDIS_PASSWORD")
	viper.BindEnv("redis.db", "REDIS_DB")
	viper.BindEnv("redis.pool_size", "REDIS_POOL_SIZE")
	viper.BindEnv("redis.max_idle", "REDIS_MAX_IDLE")
}

// handleArrayEnvVars manually handles array environment variables
func handleArrayEnvVars(config *Config) {
	// Handle RabbitMQ Queues
	if queuesStr := os.Getenv("RABBITMQ_QUEUES"); queuesStr != "" {
		config.RabbitMQ.Queues = strings.Split(queuesStr, ",")
		// Trim whitespace from each queue name
		for i, queue := range config.RabbitMQ.Queues {
			config.RabbitMQ.Queues[i] = strings.TrimSpace(queue)
		}
	}

	// Handle RabbitMQ Routing Keys
	if routingKeysStr := os.Getenv("RABBITMQ_ROUTING_KEYS"); routingKeysStr != "" {
		config.RabbitMQ.RoutingKeys = strings.Split(routingKeysStr, ",")
		// Trim whitespace from each routing key
		for i, key := range config.RabbitMQ.RoutingKeys {
			config.RabbitMQ.RoutingKeys[i] = strings.TrimSpace(key)
		}
	}

	// Handle Google OAuth2 Scopes (if needed from env)
	if scopesStr := os.Getenv("GOOGLE_OAUTH2_SCOPES"); scopesStr != "" {
		config.GoogleOAuth2.Scope = strings.Split(scopesStr, ",")
		// Trim whitespace from each scope
		for i, scope := range config.GoogleOAuth2.Scope {
			config.GoogleOAuth2.Scope[i] = strings.TrimSpace(scope)
		}
	}
}
