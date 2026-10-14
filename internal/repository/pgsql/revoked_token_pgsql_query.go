package pgsql

var (
	// QueryRevokeToken records a revoked JTI in one statement, so concurrent
	// callers cannot both insert it: exactly one sees a row affected. A repeat
	// revoke keeps the first row, whose expiry already covers the token.
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
