package pgsql

var (
	// QueryRevokeToken records a revoked JTI. A repeat revoke of the same JTI
	// keeps the first row, whose expiry already covers the token's lifetime.
	QueryRevokeToken = `
		INSERT INTO revoked_tokens (jti, expires_at) VALUES ($1, $2)
		ON CONFLICT (jti) DO NOTHING
	`

	// QueryIsTokenRevoked reports whether a JTI has an unexpired revocation.
	QueryIsTokenRevoked = `
		SELECT EXISTS (
			SELECT 1 FROM revoked_tokens WHERE jti = $1 AND expires_at > $2
		)
	`

	// QueryPurgeExpiredTokens deletes revocations whose token can no longer be used.
	QueryPurgeExpiredTokens = `
		DELETE FROM revoked_tokens WHERE expires_at <= $1
	`
)
