-- name: InsertUserIfMissing :exec
INSERT INTO users (id, phone) VALUES (@id, @phone)
ON CONFLICT (phone) DO NOTHING;

-- name: GetUser :one
SELECT * FROM users WHERE id = @id;

-- name: GetUserByPhone :one
SELECT * FROM users WHERE phone = @phone;

-- name: UpdateUser :one
-- Null leaves a field as it is; an empty email clears it. A changed email is no longer
-- confirmed. The previous email and whether it was confirmed come back too.
WITH prev AS (SELECT p.id, p.email, p.email_confirmed_at FROM users p WHERE p.id = @id FOR UPDATE)
UPDATE users u SET
    name     = coalesce(sqlc.narg('name')::text, u.name),
    email    = CASE WHEN sqlc.narg('email')::text IS NULL THEN u.email
                    ELSE nullif(sqlc.narg('email')::text, '') END,
    language = coalesce(sqlc.narg('language')::text, u.language),
    email_confirmed_at = CASE WHEN sqlc.narg('email')::text IS NULL
                                   OR nullif(sqlc.narg('email')::text, '') IS NOT DISTINCT FROM prev.email
                              THEN u.email_confirmed_at END
FROM prev
WHERE u.id = prev.id
RETURNING u.*, prev.email AS previous_email, prev.email_confirmed_at AS previous_confirmed_at;

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
