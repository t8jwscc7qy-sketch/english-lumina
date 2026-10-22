-- name: GetSettings :one
SELECT * FROM settings WHERE user_id = $1;

-- name: UpsertSettings :one
INSERT INTO settings (user_id, current_level, target_level, target_date, daily_min_minutes, timezone)
VALUES ($1, $2, $3, $4, $5, $6)
ON CONFLICT (user_id) DO UPDATE SET
    current_level     = EXCLUDED.current_level,
    target_level      = EXCLUDED.target_level,
    target_date       = EXCLUDED.target_date,
    daily_min_minutes = EXCLUDED.daily_min_minutes,
    timezone          = EXCLUDED.timezone,
    updated_at        = now()
RETURNING *;
