package pgsql

var (
	// QueryCheckByEmailOrPhoneNumber checks if a user exists with the given email or phone number
	// Returns true if a user with either the email (when not empty) or phone number (when not empty) exists
	QueryCheckByEmailOrPhoneNumber = `
		SELECT EXISTS (
			SELECT 1 FROM users 
			WHERE (email = $1 AND $1 != '') 
			   OR (phone_number = $2 AND $2 != '')
		)
	`

	// QueryGetCredentialsByEmailOrPhoneNumber gets the user's credentials (id, email, phone number, password) by email or phone number
	// Returns the user's credentials if a user with either the email (when not empty) or phone number (when not empty) exists
	// #nosec G101 -- This is a SQL query string, not hardcoded credentials
	QueryGetCredentialsByEmailOrPhoneNumber = `
		SELECT id, account_number, name, email, phone_number, phone_country_code, password FROM users
		WHERE (email = $1 AND $1 != '')
		   OR (phone_number = $2 AND $2 != '')
	`

	// QueryGetByAccountNumber gets the user's credentials (id, email, phone number, password) by account number
	// Returns the user's credentials if a user with the account number exists
	QueryGetByAccountNumber = `
		SELECT id, account_number, name, email, phone_number, phone_country_code, created_at, updated_at FROM users
		WHERE account_number = $1
	`
)
