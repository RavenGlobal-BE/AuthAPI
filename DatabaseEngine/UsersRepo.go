package databaseengine

import (
	"context"
	"errors"
	"fmt"
	logger "raven/auth/Logging"
	"time"

	"github.com/bwmarrin/snowflake"
)

type Usersrepo struct {
	db *DB
}

func NewUsersRepo(db *DB) *Usersrepo {
	return &Usersrepo{db: db}
}

// It queries the users database based on the
func (Ur *Usersrepo) GetAccountByEmail(mail string) *UserAuth {
	var v = &UserAuth{} //creates an empty struct
	row := Ur.db.pool.QueryRow(context.Background(), `select user_id, email, password, first_name, last_name, publicusername, countrycode, is_deleted, is_verified from accounts.users where email = $1`, mail)
	err := row.Scan(
		&v.UserID,
		&v.Email,
		&v.Password,
		&v.FirstName,
		&v.LastName,
		&v.PublicUsername,
		&v.CountryCode,
		&v.IsDeleted,
		&v.IsVerified,
	)

	if err != nil {
		return nil
	}

	return v
}

// It queries the users database based on the
func (Ur *Usersrepo) GetAccountById(id int) *UserAuth {
	var v = &UserAuth{} //creates an empty struct
	row := Ur.db.pool.QueryRow(context.Background(), `select user_id, email, password, first_name, last_name, publicusername, countrycode, is_verified from accounts.users where user_id = $1`, id)

	err := row.Scan(
		&v.UserID,
		&v.Email,
		&v.Password,
		&v.FirstName,
		&v.LastName,
		&v.PublicUsername,
		&v.CountryCode,
		&v.IsVerified,
	)

	if err != nil {
		return nil
	}

	return v
}

func (Ur *Usersrepo) ResetPassword(email string, password string) error {
	_, err := Ur.db.pool.Exec(context.Background(), `
		UPDATE accounts.users SET password = $1 WHERE email = $2;
	`, password, email)
	return err
}

func snowflakeFromTime(t time.Time, nodeID int64, seq int64) int64 {
	ms := t.UnixMilli() - 1748736000000
	return (ms << 22) | (nodeID << 12) | seq
}

// Puts all your details into the database
func (Ur *Usersrepo) RegisterAccount(email, password, firstName, lastName, countryCode, username string) error {
	if email == "" || username == "" || countryCode == "" || firstName == "" || lastName == "" || password == "" {
		return errors.New("Invalid request")
	}
	snowflake.Epoch = 1748736000000
	node, err := snowflake.NewNode(1)
	if err != nil {
		fmt.Println(err)
	}

	userid := node.Generate().Int64()

	_, err = Ur.db.pool.Exec(context.Background(), `
		INSERT INTO accounts.users (user_id, email, password, first_name, last_name, "publicusername", "countrycode")
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`, userid, email, password, firstName, lastName, username, countryCode)

	if err != nil {
		return err
	}

	return nil
}

func (Ur *Usersrepo) VerifyAccount(email string) error {
	_, err := Ur.db.pool.Exec(context.Background(), `
		UPDATE accounts.users SET is_verified = 1 WHERE email = $1;
	`, email)
	return err
}

func (Ur *Usersrepo) SetCountryCode(userID int64, countryCode string) error {
	if countryCode == "" || len(countryCode) > 3 {
		return errors.New("Invalid country code")
	}
	_, err := Ur.db.pool.Exec(context.Background(), `
		UPDATE accounts.users SET countrycode = $1 WHERE user_id = $2;
	`, countryCode, userID)
	return err
}

type UserAuth struct {
	UserID         int64
	Email          string
	Password       string // ArgonID2 (cost 12)
	FirstName      string
	LastName       string
	CountryCode    *string
	PublicUsername *string
	IsDeleted      int16
	IsVerified     int16
}

type User struct {
	UserID         int64
	Email          string
	Password       string // Bycrypted (cost 12)
	FirstName      string
	LastName       *string
	PublicUsername *string
	CountryCode    string
	IsDeleted      int16
	TimeDeletion   *time.Time
}

func (Ur *Usersrepo) Init() error {
	if err := Ur.db.Ping(context.Background()); err != nil {
		return err
	}

	for _, step := range []func() error{
		Ur.SetupSchema,
		Ur.SetupUsersTable,
		Ur.SetupCompanyIntegration,
	} {
		if err := step(); err != nil {
			return err
		}
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
    user_id         BIGINT PRIMARY KEY,          -- Twitter Snowflake implementation (soon)
    email           VARCHAR(255) UNIQUE NOT NULL,
    password        VARCHAR(255) NOT NULL,
    first_name      VARCHAR(255) NOT NULL,
    last_name       VARCHAR(255),                -- optional (NULL allowed)
    publicUsername  VARCHAR(255),                -- optional (NULL allowed) -> Means private account
    countryCode     VARCHAR(5) NOT NULL,
    is_deleted      SMALLINT NOT NULL DEFAULT 0,
	is_verified     SMALLINT NOT NULL DEFAULT 0,
	flags           SMALLINT NOT NULL DEFAULT 0,
    timeDeletion    TIMESTAMP,                   -- optional
	profilePicture  VARCHAR(255)                 -- optional key (NULL Allowed)
	);`, schema, table)

	_, err := Ur.db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS is_verified SMALLINT NOT NULL DEFAULT 0;`,
		schema, table,
	))

	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS countrycode VARCHAR(5) NULL;`,
		schema, table,
	))

	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS profilePicture VARCHAR(255) NULL;`,
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
		created_at    TIMESTAMP NOT NULL DEFAULT NOW(),
		company_icon  VARCHAR(255) DEFAULT NULL
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

	_, _ = Ur.db.pool.Exec(context.Background(), fmt.Sprintf(
		`ALTER TABLE %s.%s ADD COLUMN IF NOT EXISTS company_icon VARCHAR(255) DEFAULT NULL;`,
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
