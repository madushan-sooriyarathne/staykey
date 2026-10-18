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

-- name: RenameAccount :one
UPDATE accounts SET name = @name WHERE id = @id RETURNING *;

-- Accounts are owned by the person who created them; invites only add managers and caretakers.
-- name: OwnedAccountIDs :many
SELECT m.account_id FROM memberships m WHERE m.user_id = @user_id AND m.role = 'owner' ORDER BY m.account_id;

-- name: CloseAccount :exec
UPDATE accounts SET status = 'closed', closed_at = @now::timestamptz WHERE id = @id;

-- Reopened accounts start from trial; the caller works out their real status straight after.
-- name: ReopenOwnedAccounts :many
UPDATE accounts SET status = 'trial', closed_at = NULL
WHERE status = 'closed' AND closed_at IS NOT NULL
  AND id IN (SELECT m.account_id FROM memberships m WHERE m.user_id = @user_id AND m.role = 'owner')
RETURNING id;

-- name: DeleteTeamMemberships :exec
DELETE FROM memberships WHERE user_id = @user_id AND role <> 'owner';

-- name: DeleteClosedOwnedAccounts :exec
DELETE FROM accounts
WHERE status = 'closed'
  AND id IN (SELECT m.account_id FROM memberships m WHERE m.user_id = @user_id AND m.role = 'owner');
