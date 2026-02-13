package databaseengine

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	logger "raven/auth/Logging"

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

var DbPointer *sql.DB

func PGConnect() error {
	dsn := "postgres://postgres:6464@localhost:5432/postgres?sslmode=disable"
	db, err := sql.Open("postgres", dsn)

	DbPointer = db

	DbPointer.SetMaxOpenConns(25)
	DbPointer.SetMaxIdleConns(25)
	DbPointer.SetConnMaxLifetime(5 * time.Minute)

	if err != nil {
		return err
	}

	return nil
}

func PGQuery(destination any) {
	if DbPointer == nil {
		logger.Log("Null pointer found. Reconnecting to DB...", logger.Error)
		err := PGConnect()

		if err != nil {
			logger.Log(err.Error(), logger.Fatal)
		} else {
			logger.Log("DB connected successfully", logger.Pass)
		}
	}

	row, err := DbPointer.Query("select * from mobileroaming.carrier_info where id = 12")
	if err != nil {
		log.Fatal(err)
	}

	defer func(row *sql.Rows) {
		err := row.Close()
		if err != nil {
			logger.Log(err.Error(), logger.Error)
		}
	}(row)

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
