-- name: UserExists :one
SELECT EXISTS (SELECT 1 FROM users);

-- name: CreateUser :one
INSERT INTO users (login, password_hash)
VALUES ($1, $2)
RETURNING *;

-- name: GetUserByLogin :one
SELECT * FROM users WHERE login = $1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = $1;

-- name: UpdatePasswordHash :exec
UPDATE users
SET password_hash = $2, password_changed_at = now(), updated_at = now()
WHERE id = $1;
