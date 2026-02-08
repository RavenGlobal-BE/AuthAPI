package databaseengine

import (
	"database/sql"
	"fmt"
	"time"

	logging "raven/auth/Logging"

	"github.com/go-sql-driver/mysql"
	//"github.com/lib/pq"
)

var Db *sql.DB // Database pointer -> Shared across the entire program

func GetQuery(query string, destination any, args ...any) error { //Gets data from the database tables
	switch v := destination.(type) {
	case *Users:
		err := Db.QueryRow(query, args...).Scan(
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

func InsertQuery(query string, args ...any) (string, error) {
	result, err := Db.Exec(query, args...)

	if err != nil {
		fmt.Println("Error executing insert query:", err)
		return "", err
	}

	lastInsertID, err := result.LastInsertId()
	if err != nil {
		fmt.Println("Error fetching last insert ID:", err)
		return "", err
	}

	return fmt.Sprintf("%d", lastInsertID), nil
}

func ConnectDB(DBname string) {
	cfg := mysql.NewConfig()
	cfg.User = "root"
	cfg.Passwd = ""
	cfg.Net = "tcp"
	cfg.Addr = "127.0.0.1:3306"
	cfg.DBName = DBname

	var err error
	Db, err = sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}

	Db.SetMaxOpenConns(30)
	Db.SetConnMaxIdleTime(5 * time.Minute)

	pingErr := Db.Ping()
	if pingErr != nil {
		logging.Log("Database connection failed", logging.Fatal)
	}

	prepareDB()
}

func prepareDB() { //Checks whether the database connection has the correct structure
	Db.Query("CREATE TABLE IF NOT EXISTS sessions (token VARCHAR(255) PRIMARY KEY, user_id INT, created_at TIMESTAMP, expires_at TIMESTAMP)")
}
