-- name: InsertUserIfMissing :exec
INSERT INTO users (id, phone) VALUES (@id, @phone)
ON CONFLICT (phone) DO NOTHING;

-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = @phone;

-- name: UpdateUser :one
-- Null leaves a field as it is; an empty email clears it.
UPDATE users SET
    name     = coalesce(sqlc.narg('name')::text, name),
    email    = CASE WHEN sqlc.narg('email')::text IS NULL THEN email
                    ELSE nullif(sqlc.narg('email')::text, '') END,
    language = coalesce(sqlc.narg('language')::text, language)
WHERE id = @id
RETURNING *;
