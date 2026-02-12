package databaseengine

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	logging "raven/auth/Logging"

	"github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
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

func InsertQuery(query string, args ...any) (int64, error) { //Returns the number of affected rows
	result, err := Db.Exec(query, args...)

	if err != nil {
		fmt.Println("Error executing insert query:", err)
		return 0, err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		fmt.Println("Error fetching rows affected:", err)
		return 0, err
	}

	return rowsAffected, nil
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
	_, err := Db.Query("CREATE TABLE IF NOT EXISTS sessions (token VARCHAR(512) PRIMARY KEY, user_id INT, created_at TIMESTAMP, expires_at TIMESTAMP, refresh_token VARCHAR(512), refresh_expires_at TIMESTAMP)")
	if err != nil {
		logging.Log(err.Error(), logging.Fatal)
	}
}

var DbPointer *sql.DB

func PGConnect(destination any) {
	dsn := "postgres://postgres:6464@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("postgres", dsn)

	DbPointer = db

	if err != nil {
		log.Fatal(err)
	}
}

func PGQuery(destination any) {
	if DbPointer == nil {
		logging.Log("Null pointer exception. Terminating the program now...", logging.Fatal)
	}

	row, err := DbPointer.Query("select * from mobileroaming.carrier_info where id = 12")
	if err != nil {
		log.Fatal(err)
	}

	switch v := destination.(type) {
	case *MobileInfo:
		if row.Next() {
			err = row.Scan(
				&v.Id,
				&v.Mcc,
				&v.Mnc,
				&v.OperatorName,
				&v.TwoGShutdown,
				&v.ThreeGShutdown,
				&v.LTEShutdown,
				&v.CountryCode,
				&v.active,
			)

			if err != nil {
				log.Fatal(err)
			}
		}
	default:
		fmt.Println(destination)
	}
}
