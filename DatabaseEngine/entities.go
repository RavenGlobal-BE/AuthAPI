package databaseengine

type Sessions struct { // Structure for the Accounts.Sessions table
	token      string
	user_id    int16 // User accounts -> Up to 32.767 users
	created_at string
	expires_at string
}

type Accounts struct { // Structure for the Accounts.Users table
	id                 int16 // User accounts -> Up to 32.767 users
	email              string
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
