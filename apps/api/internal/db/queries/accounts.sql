-- name: CreateAccount :one
INSERT INTO accounts (id, name) VALUES (@id, @name)
RETURNING *;

-- name: DeleteAccount :exec
DELETE FROM accounts WHERE id = @id;

-- name: CreateMembership :one
INSERT INTO memberships (id, account_id, user_id, role) VALUES (@id, @account_id, @user_id, @role)
RETURNING *;

-- name: AddMembershipProperty :exec
INSERT INTO membership_properties (account_id, membership_id, property_id)
VALUES (@account_id, @membership_id, @property_id);

-- name: ListAccountsForUser :many
SELECT a.id, a.name, a.status, a.trial_started_at, a.created_at,
       m.id AS membership_id, m.role,
       coalesce(array_agg(mp.property_id ORDER BY mp.property_id)
                FILTER (WHERE mp.property_id IS NOT NULL), '{}')::uuid[] AS property_ids
FROM memberships m
JOIN accounts a ON a.id = m.account_id
LEFT JOIN membership_properties mp ON mp.membership_id = m.id
WHERE m.user_id = @user_id AND m.status = 'active' AND a.status <> 'closed'
GROUP BY a.id, m.id
ORDER BY m.created_at, m.id;

-- name: GetMembership :one
-- The tenant lookup behind every owner route.
SELECT m.id, m.account_id, m.user_id, m.role,
       coalesce(array_agg(mp.property_id ORDER BY mp.property_id)
                FILTER (WHERE mp.property_id IS NOT NULL), '{}')::uuid[] AS property_ids
FROM memberships m
JOIN accounts a ON a.id = m.account_id
LEFT JOIN membership_properties mp ON mp.membership_id = m.id
WHERE m.user_id = @user_id AND m.account_id = @account_id
  AND m.status = 'active' AND a.status <> 'closed'
GROUP BY m.id;

-- name: CreateInvite :exec
INSERT INTO invites (id, account_id, phone, name, role, property_ids, token_hash, invited_by, expires_at)
VALUES (@id, @account_id, @phone, @name, @role, @property_ids, @token_hash, @invited_by, @expires_at);
