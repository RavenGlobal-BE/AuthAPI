package databaseengine

import (
	"context"
	"fmt"
	"sync"

	"time"

	logger "raven/auth/Logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

var NewDbPointer *pgxpool.Pool

func NewPGConnect() error {
	connString := "postgres://postgres:6464@localhost:5432/postgres?sslmode=disable"
	config, err := pgxpool.ParseConfig(connString)

	if err != nil {
		logger.Log("Unable to parse config: "+err.Error(), logger.Fatal)
		return err
	}

	config.MaxConnIdleTime = 5 * time.Minute
	config.MaxConns = 50
	config.MinConns = 1

	newPool, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		logger.Log("Unable to connect to database: "+err.Error(), logger.Fatal)
		return err
	}

	NewDbPointer = newPool
	return nil
}

func PGQueryCarrierInfo(ctx context.Context, infoStruct *MobileInfo) {

}

func PGQuery(ctx context.Context, destination any, ch chan<- any, wg *sync.WaitGroup) error {
	if NewDbPointer == nil {
		logger.Log("Null pointer found. Reconnecting to DB...", logger.Error)
		err := NewPGConnect()

		if err != nil {
			logger.Log(err.Error(), logger.Fatal)
			return err
		} else {
			logger.Log("DB connected successfully", logger.Pass)
		}
	}

	switch v := destination.(type) {
	case *MobileInfo:
		row := NewDbPointer.QueryRow(ctx, "SELECT * FROM mobileroaming.carrier_info WHERE id = $1", 12)
		err := row.Scan(&v.Id, &v.Mcc, &v.Mnc, &v.OperatorName, &v.TwoGShutdown, &v.ThreeGShutdown, &v.LTEShutdown, &v.CountryCode, &v.active)

		if err != nil {
			logger.Log("Query error: "+err.Error(), logger.Error)
			return err
		}
	default:
		fmt.Println(destination)
	}

	return nil
}

func PrepareDB() {
	fmt.Println("Preparing DB...")
}
