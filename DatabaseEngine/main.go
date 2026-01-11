package databaseengine

import (
	"database/sql"
	"log"

	"github.com/go-sql-driver/mysql"
)

var db *sql.DB // Database pointer -> Shared across the entire program

type Sessions struct { // Structure for the Accounts.Sessions table
	token      string
	user_id    int16 // User accounts -> Up to 32.767 users
	created_at string
	expires_at string
}

type Accounts struct { // Structure for the Accounts.Sessions table
	id                 int16
	email              string // User accounts -> Up to 32.767 users
	hashed_password    string
	verification_token *string // Optional field for email verification
	is_verified        int8    // 0 = false, 1 = true
	is_banned          int8    // 0 = false, 1 = true
	firstName          string
	lastName           string
	eSIMSerial         *string // Optional field for eSIM serial number
	isDeleted          int8    // 0 = false, 1 = true
	schdeletedDeletion *string
	stripeCustomerID   *string // Optional field for Stripe customer ID
}

func ExecuteQuery(query string, T any) {
	db.Exec(query)
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

	pingErr := db.Ping()
	if pingErr != nil {
		log.Fatal(pingErr)
	}
}
