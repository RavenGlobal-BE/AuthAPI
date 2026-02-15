package databaseengine

import "time"

type Sessions struct { // Structure for the Accounts.Sessions table
	token      string
	user_id    int16 // User accounts -> Up to 32.767 users
	created_at string
	expires_at string
}

type MobileInfo struct {
	Id             int16
	Mcc            string
	Mnc            string
	OperatorName   string
	TwoGShutdown   time.Time
	ThreeGShutdown time.Time
	LTEShutdown    time.Time
	CountryCode    string
	active         int
}
