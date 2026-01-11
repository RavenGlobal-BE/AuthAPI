package databaseengine

type Sessions struct { // Structure for the Accounts.Sessions table
	token      string
	user_id    int16 // User accounts -> Up to 32.767 users
	created_at string
	expires_at string
}

type Users struct { // Structure for the Accounts.Users table
	Id                 int16 // User accounts -> Up to 32.767 users
	Email              string
	Hashed_password    string
	Verification_token *string // Optional field for email verification
	Is_verified        int8    // 0 = false, 1 = true
	Created_at         string
	Is_banned          int8 // 0 = false, 1 = true
	FirstName          string
	LastName           string
	ESIMSerial         *string // Optional field for eSIM serial number
	NewParticleAllowed int8    // 0 = false, 1 = true
	IsDeleted          int8    // 0 = false, 1 = true
	SchdeletedDeletion *string
	StripeCustomerID   *string // Optional field for Stripe customer ID
}
