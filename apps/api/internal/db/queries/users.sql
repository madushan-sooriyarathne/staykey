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

-- name: MarkUserDeleted :exec
UPDATE users SET deleted_at = @now::timestamptz WHERE id = @id;

-- name: RestoreUser :execrows
UPDATE users SET deleted_at = NULL WHERE id = @id AND deleted_at IS NOT NULL;

-- name: UsersDeletedBefore :many
SELECT id FROM users WHERE deleted_at < @before::timestamptz ORDER BY id;

-- name: DeleteUser :exec
DELETE FROM users WHERE id = @id;

-- name: SetUserPhoto :exec
UPDATE users SET photo_key = sqlc.narg('photo_key') WHERE id = @id;
