-- name: CreateSession :one
INSERT INTO sessions (user_id, token_hash, expires_at, user_agent)
VALUES ($1, $2, $3, $4)
RETURNING *;

-- name: GetActiveSession :one
SELECT * FROM sessions
WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now();

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = now()
WHERE id = $1 AND last_seen_at < now() - interval '5 minutes';

-- name: RevokeSession :exec
UPDATE sessions SET revoked_at = now()
WHERE token_hash = $1 AND revoked_at IS NULL;

-- name: RevokeUserSessions :exec
UPDATE sessions SET revoked_at = now()
WHERE user_id = $1 AND revoked_at IS NULL;

-- name: DeleteDeadSessions :execrows
DELETE FROM sessions
WHERE expires_at <= now() OR revoked_at IS NOT NULL;
