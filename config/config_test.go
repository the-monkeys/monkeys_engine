package config

import (
	"testing"

	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

const config = `
the_monkeys_gateway:
	HTTPS: 0.0.0.0:8080
	HTTP: 0.0.0.0:8081
microservices:
	the_monkeys_authz: 127.0.0.1:50051
	the_monkeys_blog: 127.0.0.1:50052
	the_monkeys_user: 127.0.0.1:50053
	the_monkeys_file_storage: 127.0.0.1:50054
postgresql:
	primary_db:
		db_username: root
		db_password: Secret
		db_host: 0.0.0.0
		db_port: 5432
		db_name: the_monkeys_db_dev
	replica_1:
		db_username: root
		db_password: Secret
		db_host: 0.0.0.0
		db_port: 5432
		db_name: the_monkeys_db_dev
jwt:
	secret_key: Secret
opensearch:
	address: https://localhost:9200	
	os_username: admin
	os_password: admin
email:
	smtp_address: ""
	smtp_mail: "" 
	smtp_password: ""
	smtp_host: ""
`

func TestGetConfig(t *testing.T) {
	t.Run("Success", func(t *testing.T) {
		// Arrange
		viper.SetConfigName("config")
		viper.SetConfigType("yml")
		viper.AddConfigPath(".")

		// Act
		config, err := GetConfig()

		// Assert
		assert.NoError(t, err)
		assert.NotNil(t, config)
	})
}
