package config

import (
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
)

type TheMonkeysGateway struct {
	HTTPS string `mapstructure:"HTTPS"`
	HTTP  string `mapstructure:"HTTP"`
}

type Microservices struct {
	TheMonkeysAuthz        string `mapstructure:"the_monkeys_authz"`
	TheMonkeysBlog         string `mapstructure:"the_monkeys_blog"`
	TheMonkeysUser         string `mapstructure:"the_monkeys_user"`
	TheMonkeysFileStore    string `mapstructure:"the_monkeys_storage"`
	TheMonkeysNotification string `mapstructure:"the_monkeys_notification"`
	TheMonkeysCache        string `mapstructure:"the_monkeys_cache"`
}

type Database struct {
	DBUsername string `mapstructure:"db_username"`
	DBPassword string `mapstructure:"db_password"`
	DBHost     string `mapstructure:"db_host"`
	DBPort     int    `mapstructure:"db_port"`
	DBName     string `mapstructure:"db_name"`
}

type Postgresql struct {
	PrimaryDB Database `mapstructure:"primary_db"`
	Replica1  Database `mapstructure:"replica_1"`
}

type JWT struct {
	SecretKey string `mapstructure:"secret_key"`
}

type Opensearch struct {
	Address  string `mapstructure:"address"`
	Host     string `mapstructure:"os_host"`
	Username string `mapstructure:"os_username"`
	Password string `mapstructure:"os_password"`
}

type Email struct {
	SMTPAddress  string `mapstructure:"smtp_address"`
	SMTPMail     string `mapstructure:"smtp_mail"`
	SMTPPassword string `mapstructure:"smtp_password"`
	SMTPHost     string `mapstructure:"smtp_host"`
}

type Authentication struct {
	EmailVerificationAddr string `mapstructure:"email_verification_addr"`
}

type RabbitMQ struct {
	Protocol    string   `mapstructure:"protocol"`
	Host        string   `mapstructure:"host"`
	Port        string   `mapstructure:"port"`
	Username    string   `mapstructure:"username"`
	Password    string   `mapstructure:"password"`
	VirtualHost string   `mapstructure:"virtual_host"`
	Exchange    string   `mapstructure:"exchange"`
	Queues      []string `yaml:"queues"`
	RoutingKeys []string `yaml:"routingKeys"`
}

type Keys struct {
	MediaStack     string `mapstructure:"mediastack"`
	NewsApi        string `mapstructure:"newsapi"`
	HindustanTimes string `mapstructure:"hindustantimes"`
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
	Authentication    Authentication    `mapstructure:"authentication"`
	RabbitMQ          RabbitMQ          `mapstructure:"rabbitMQ"`
	Keys              Keys              `mapstructure:"keys"`
	GoogleOAuth2      GoogleOAuth2      `mapstructure:"google_oauth2"`
	Cors              Cors              `mapstructure:"cors"`
	Redis             Redis             `mapstructure:"redis"`
}

func GetConfig() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml") // Ensure this is yaml
	viper.AddConfigPath("./config")

	viper.AutomaticEnv() // Ensures environment variables are loaded
	config := &Config{}

	// Binding struct to config
	if err := viper.ReadInConfig(); err != nil {
		logrus.Errorf("Error reading config file, %v", err)
		return config, err
	}

	if err := viper.Unmarshal(config); err != nil {
		logrus.Errorf("Unable to decode into struct, %v", err)
		return config, err
	}

	return config, nil
}
