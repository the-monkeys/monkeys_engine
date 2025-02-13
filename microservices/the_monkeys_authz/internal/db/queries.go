package db

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/the-monkeys/the_monkeys/config"
	"github.com/the-monkeys/the_monkeys/microservices/the_monkeys_authz/internal/models"
)

type Repository interface {
	// Get operation
	GetUserByEmail(email string)
	GetUserByUsername(username string)
	GetUserByAccountId(accountID string) (*models.TheMonkeysAccount, error)

	// Create operation
	RegisterUser()
	CreateAUserLog()

	// Update operation
	UpdatePasswordRecoveryToken()
	UpdatePassword()
	UpdateEmailVerificationToken()
	UpdateEmailVerificationStatus()
}

type postgresDB struct {
	db  *sql.DB
	log *logrus.Logger
}

func NewAuthPostgresDb(cfg *config.Config, log *logrus.Logger) (Repository, error) {
	url := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable",
		cfg.Postgresql.PrimaryDB.DBUsername,
		cfg.Postgresql.PrimaryDB.DBPassword,
		cfg.Postgresql.PrimaryDB.DBHost,
		cfg.Postgresql.PrimaryDB.DBPort,
		cfg.Postgresql.PrimaryDB.DBName,
	)

	dbPsql, err := sql.Open("postgres", url)
	if err != nil {
		logrus.Fatalf("cannot connect psql using sql driver, error:, %+v", err)
		return nil, err
	}
	// Configure connection pooling
	dbPsql.SetMaxOpenConns(25)                 // Maximum number of open connections
	dbPsql.SetMaxIdleConns(10)                 // Maximum number of idle connections
	dbPsql.SetConnMaxLifetime(5 * time.Minute) // Connection lifetime limit

	if err = dbPsql.Ping(); err != nil {
		logrus.Errorf("ping test failed to psql using sql driver, error: %+v", err)
		return nil, err

	}

	return &postgresDB{db: dbPsql, log: log}, nil
}

func (auth *postgresDB) GetUserByEmail(email string) {}

func (auth *postgresDB) GetUserByUsername(username string) {}

func (auth *postgresDB) GetUserByAccountId(accountID string) (*models.TheMonkeysAccount, error) {
	var tma models.TheMonkeysAccount
	if err := auth.db.QueryRow(`SELECT id, account_id, username, first_name, last_name, email 
	FROM user_accounts where account_id=$1;`, accountID).Scan(&tma.Id,
		&tma.AccountId, &tma.Username, &tma.FirstName, &tma.LastName, &tma.Email); err != nil {
		auth.log.Errorf("can't find a user existing with account id %s, error: %+v", accountID, err)
		return nil, err
	}

	return &tma, nil
}

func (auth *postgresDB) RegisterUser()   {}
func (auth *postgresDB) CreateAUserLog() {}

func (auth *postgresDB) UpdatePasswordRecoveryToken()   {}
func (auth *postgresDB) UpdatePassword()                {}
func (auth *postgresDB) UpdateEmailVerificationToken()  {}
func (auth *postgresDB) UpdateEmailVerificationStatus() {}
