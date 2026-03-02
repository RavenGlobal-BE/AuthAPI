package databaseengine

import (
	"context"
	"fmt"
	logger "raven/auth/Logging"
	"time"
)

type Usersrepo struct {
	db *DB
}

func NewUsersRepo(db *DB) *Usersrepo {
	return &Usersrepo{db: db}
}

func (Ur *Usersrepo) Init() error {
	err := Ur.SetupSchema()
	if err != nil {
		return err
	}
	err = Ur.SetupUsersTable()
	if err != nil {
		return err
	}

	err = Ur.SetupCompanyIntegration()
	if err != nil {
		return err
	}

	return nil
}

func (Ur *Usersrepo) SetupUsersTable() error {
	// This function checks wheter the table contains all the elements used in this API.
	// If it doesn't, it creates the missing elements.

	schema := "accounts"
	table := "users"

	query := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
    user_id         SERIAL PRIMARY KEY,          -- auto-incrementable key
    email           VARCHAR(255) UNIQUE NOT NULL,
    password        VARCHAR(255) NOT NULL,
    first_name      VARCHAR(255) NOT NULL,
    last_name       VARCHAR(255),                -- optional (NULL allowed)
    publicUsername  VARCHAR(255),                -- optional (NULL allowed) -> Means private account
    countryCode     VARCHAR(5) NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    is_deleted      SMALLINT NOT NULL DEFAULT 0,
	is_verified     SMALLINT NOT NULL DEFAULT 0,
    timeDeletion    TIMESTAMP                    -- optional
	);`, schema, table)

	_, err := Ur.db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS is_verified SMALLINT NOT NULL DEFAULT 0;`,
		schema, table,
	))

	return nil
}

func (Ur *Usersrepo) SetupCompanyIntegration() error {
	schema := "accounts"
	table := "apps"

	query := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
		app_id        SERIAL PRIMARY KEY,
		client_id     VARCHAR(255) UNIQUE NOT NULL,
		client_secret VARCHAR(255) NOT NULL,
		app_name      VARCHAR(255) NOT NULL,
		company       VARCHAR(255) NOT NULL,
		redirect_uri  VARCHAR(255) NOT NULL,
		is_public     BOOLEAN NOT NULL DEFAULT false,
		active        BOOLEAN NOT NULL DEFAULT true,
		created_at    TIMESTAMP NOT NULL DEFAULT NOW()
	);`, schema, table)

	_, err := Ur.db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	// Add is_public to existing tables that were created before this column existed
	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS is_public BOOLEAN NOT NULL DEFAULT false;`,
		schema, table,
	))

	logger.Log("3rdParty Apps table ready", logger.Info)

	return nil
}

func (Ur *Usersrepo) SetupSchema() error {
	schema := "accounts"

	query := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, schema)

	_, err := Ur.db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	return nil
}

// It queries the users database based on the
func (Ur *Usersrepo) GetAccountByEmail(mail string) *UserAuth {
	var v = &UserAuth{} //creates an empty struct
	row := Ur.db.pool.QueryRow(context.Background(), `select user_id, email, password, first_name, last_name, created_at, is_deleted from accounts.users where email = $1`, mail)
	err := row.Scan(
		&v.UserID,
		&v.Email,
		&v.Password,
		&v.FirstName,
		&v.LastName,
		&v.CreatedAt,
		&v.IsDeleted,
	)

	if err != nil {
		return nil
	}

	return v
}

// It queries the users database based on the
func (Ur *Usersrepo) GetAccountById(id int) *UserAuth {
	var v = &UserAuth{} //creates an empty struct
	row := Ur.db.pool.QueryRow(context.Background(), `select user_id, email, password, first_name, last_name, created_at, is_deleted from accounts.users where user_id = $1`, id)
	err := row.Scan(
		&v.UserID,
		&v.Email,
		&v.Password,
		&v.FirstName,
		&v.LastName,
		&v.CreatedAt,
		&v.IsDeleted,
	)

	if err != nil {
		return nil
	}

	return v
}

type UserAuth struct {
	UserID    int32
	Email     string
	Password  string // Bycrypted (cost 12)
	FirstName string
	LastName  string
	CreatedAt time.Time
	IsDeleted int16
}

type User struct {
	UserID         int32
	Email          string
	Password       string // Bycrypted (cost 12)
	FirstName      string
	LastName       *string
	PublicUsername *string
	CountryCode    string
	CreatedAt      time.Time
	IsDeleted      int16
	TimeDeletion   *time.Time
}
