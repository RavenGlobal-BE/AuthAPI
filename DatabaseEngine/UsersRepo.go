package databaseengine

import (
	"context"
	"fmt"
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

	_, err := Ur.db.pool.Exec(context.Background(), query)
	if err != nil {
		return err
	}

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
func (Ur *Usersrepo) GetAccountByEmail() *User {
	var v = &User{} //creates an empty struct
	row := Ur.db.pool.QueryRow(context.Background(), `select * from accounts.users where email = $1`, "imad@raven.co.com")
	err := row.Scan(
		&v.UserID,
		&v.Email,
		&v.Password,
		&v.FirstName,
		&v.LastName,
		&v.PublicUsername,
		&v.CountryCode,
		&v.CreatedAt,
		&v.IsDeleted,
		&v.TimeDeletion,
	)

	if err != nil {
		fmt.Println("Error scanning row: ", err)
		return nil
	}

	return v
}

type User struct {
	UserID         int32
	Email          string
	Password       string // Bycrypted (cost 14)
	FirstName      string
	LastName       *string
	PublicUsername *string
	CountryCode    string
	CreatedAt      time.Time
	IsDeleted      int16
	TimeDeletion   *time.Time
}
