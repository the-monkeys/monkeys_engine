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
	TheMonkeysAuthz     string `mapstructure:"the_monkeys_authz"`
	TheMonkeysBlog      string `mapstructure:"the_monkeys_blog"`
	TheMonkeysUser      string `mapstructure:"the_monkeys_user"`
	TheMonkeysFileStore string `mapstructure:"the_monkeys_file_storage"`
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
	Protocol    string `mapstructure:"protocol"`
	Host        string `mapstructure:"host"`
	Port        string `mapstructure:"port"`
	Username    string `mapstructure:"username"`
	Password    string `mapstructure:"password"`
	VirtualHost string `mapstructure:"virtual_host"`
	Exchange    string `mapstructure:"exchange"`
	Queue       string `mapstructure:"queue"`
	RoutingKey  string `mapstructure:"routing_key"`
}

type Config struct {
	TheMonkeysGateway TheMonkeysGateway `mapstructure:"the_monkeys_gateway"`
	Microservices     Microservices     `mapstructure:"microservices"`
	Postgresql        Postgresql        `mapstructure:"postgresql"`
	JWT               JWT               `mapstructure:"jwt"`
	Opensearch        Opensearch        `mapstructure:"opensearch"`
	Email             Email             `mapstructure:"email"`
	Authentication    Authentication    `mapstructure:"authentication"`
	RabbitMQ          RabbitMQ          `mapstructure:"rabbitmq"`
}

// TODO: remove the print statement and add logger instead
func GetConfig() (*Config, error) {
	viper.SetConfigName("config")   // name of config file (without extension)
	viper.SetConfigType("yml")      // REQUIRED if the config file does not have the extension in the name
	viper.AddConfigPath("./config") // path to look for the config file in

	if err := viper.ReadInConfig(); err != nil {
		logrus.Errorf("Error reading config file, %v", err)
		return nil, err
	}

	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		logrus.Errorf("Unable to decode into struct, %v", err)
		return nil, err
	}

	return config, nil
}
