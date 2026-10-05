-- Idempotency keys. Claiming a key inserts its row inside the request's transaction, so a
-- retry that arrives while the first attempt runs waits for it and then sees its reply.

-- name: ClaimIdempotencyKey :execrows
INSERT INTO idempotency_keys (account_id, key, request_hash, expires_at)
VALUES (@account_id, @key, @request_hash, @expires_at)
ON CONFLICT (account_id, key) DO NOTHING;

-- name: GetIdempotencyKey :one
SELECT request_hash, status, response, expires_at
FROM idempotency_keys
WHERE account_id = @account_id AND key = @key
FOR UPDATE;

-- name: DeleteIdempotencyKey :exec
DELETE FROM idempotency_keys WHERE account_id = @account_id AND key = @key;

-- name: SaveIdempotentResponse :exec
UPDATE idempotency_keys SET status = @status, response = @response
WHERE account_id = @account_id AND key = @key;

-- name: DeleteExpiredIdempotencyKeys :execrows
DELETE FROM idempotency_keys WHERE expires_at < @now::timestamptz;
