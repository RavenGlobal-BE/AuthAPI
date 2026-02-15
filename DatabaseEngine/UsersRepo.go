package databaseengine

import (
	"context"
	"fmt"
	"time"
)

type Usersrepo struct {
	db *DB
}

func (Ur Usersrepo) Init(db *DB) error {
	err := Ur.SetupSchema(db)

	err = Ur.SetupUsersTable(db)
	if err != nil {
		return err
	}

	if err != nil { //If an error happens, a 2nd attempt is made

		err := Ur.SetupSchema(db)
		if err != nil {
			return err
		}

		err = Ur.SetupUsersTable(db)
		if err != nil {
			return err
		}
	}

	return nil
}

func (Ur Usersrepo) SetupUsersTable(db *DB) error {
	// This function checks wheter the table contains all the elements used in this API.
	// If it doesn't, it creates the missing elements.

	schema := "accounts"
	table := "users"

	query := fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s.%s (
    user_id         SERIAL PRIMARY KEY,          -- auto-incrementable key
    email           VARCHAR(255) NOT NULL,
    password        VARCHAR(255) NOT NULL,
    first_name      VARCHAR(255) NOT NULL,
    last_name       VARCHAR(255),                -- optional (NULL allowed)
    publicUsername  VARCHAR(255),                -- optional (NULL allowed) -> Means private account
    countryCode     VARCHAR(5) NOT NULL,
    created_at      TIMESTAMP NOT NULL DEFAULT NOW(),
    is_deleted      SMALLINT NOT NULL DEFAULT 0,
    timeDeletion    TIMESTAMP                     -- optional
	);`, schema, table)

	_, err := db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	return nil
}

func (Ur Usersrepo) SetupSchema(db *DB) error {
	schema := "accounts"

	query := fmt.Sprintf(`CREATE SCHEMA IF NOT EXISTS %s;`, schema)

	_, err := db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

	return nil
}

type User struct {
	UserID         int32
	Email          string
	Password       string
	FirstName      string
	LastName       *string
	PublicUsername *string
	CountryCode    string
	CreatedAt      time.Time
	IsDeleted      int16
	TimeDeletion   *time.Time
}
