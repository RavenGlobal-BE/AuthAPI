package databaseengine

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB // Database pointer -> Shared across the entire program

func ExecuteQuery(query string, destination any, args ...any) error {
	switch v := destination.(type) {
	case *Users:
		err := db.QueryRow(query, args...).Scan(
			&v.Id,
			&v.Email,
			&v.Hashed_password,
			&v.Verification_token,
			&v.Is_verified,
			&v.Created_at,
			&v.Is_banned,
			&v.FirstName,
			&v.LastName,
			&v.ESIMSerial,
			&v.NewParticleAllowed,
			&v.IsDeleted,
			&v.SchdeletedDeletion,
			&v.StripeCustomerID,
		)
		if err != nil {
			fmt.Println("Error executing query:", err)
			return err
		}

		return nil

	default:
		return nil
	}
}

func ConnectDB() {
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Passwd = ""
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:3306"
	cfg.DBName = "Accounts"

	var err error
	db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		log.Fatal(err)
	}

	db.SetMaxOpenConns(30)
	db.SetConnMaxIdleTime(5 * time.Minute)

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
}
