-- Team members and invites. Every write is scoped to the account.

-- name: ListMembers :many
SELECT m.id, m.user_id, m.role, m.created_at, u.name, u.phone,
       coalesce(array_agg(mp.property_id ORDER BY mp.property_id)
                FILTER (WHERE mp.property_id IS NOT NULL), '{}')::uuid[] AS property_ids
FROM memberships m
JOIN users u ON u.id = m.user_id
LEFT JOIN membership_properties mp ON mp.membership_id = m.id
WHERE m.account_id = @account_id AND m.status = 'active'
GROUP BY m.id, u.id
ORDER BY array_position(ARRAY['owner', 'manager', 'caretaker'], m.role), m.created_at, m.id;

-- name: GetMemberForUpdate :one
SELECT id, user_id, role FROM memberships
WHERE id = @id AND account_id = @account_id AND status = 'active'
FOR UPDATE;

-- name: UpdateMemberRole :exec
UPDATE memberships SET role = @role WHERE id = @id AND account_id = @account_id;

-- name: DeleteMemberProperties :exec
DELETE FROM membership_properties WHERE membership_id = @membership_id AND account_id = @account_id;

-- name: DeleteMember :exec
DELETE FROM memberships WHERE id = @id AND account_id = @account_id;

-- name: DeleteMemberPrefs :exec
DELETE FROM notification_prefs WHERE account_id = @account_id AND user_id = @user_id;

-- name: ListOpenInvites :many
SELECT * FROM invites
WHERE account_id = @account_id AND accepted_at IS NULL AND revoked_at IS NULL
  AND expires_at > @now::timestamptz
ORDER BY created_at DESC, id DESC;

-- An open invite found by its token, with what the join screen shows.
-- name: GetOpenInvite :one
SELECT sqlc.embed(i), a.name AS account_name, coalesce(u.name, '')::text AS invited_by_name
FROM invites i
JOIN accounts a ON a.id = i.account_id
LEFT JOIN users u ON u.id = i.invited_by
WHERE i.token_hash = @token_hash AND i.accepted_at IS NULL AND i.revoked_at IS NULL
  AND i.expires_at > @now::timestamptz AND a.status <> 'closed';

-- name: GetOpenInviteForUpdate :one
SELECT i.* FROM invites i
JOIN accounts a ON a.id = i.account_id
WHERE i.token_hash = @token_hash AND i.accepted_at IS NULL AND i.revoked_at IS NULL
  AND i.expires_at > @now::timestamptz AND a.status <> 'closed'
FOR UPDATE OF i;

-- name: MarkInviteAccepted :exec
UPDATE invites SET accepted_at = @now::timestamptz, accepted_by = @user_id WHERE id = @id;

-- name: RevokeInvite :one
UPDATE invites SET revoked_at = @now::timestamptz
WHERE id = @id AND account_id = @account_id AND accepted_at IS NULL AND revoked_at IS NULL
RETURNING id;

-- name: RevokeOpenInvitesForPhone :exec
UPDATE invites SET revoked_at = @now::timestamptz
WHERE account_id = @account_id AND phone = @phone AND accepted_at IS NULL AND revoked_at IS NULL;

-- Property names for the join screen. No ids means every property in the account.
-- name: PropertyNames :many
SELECT name FROM properties
WHERE account_id = @account_id
  AND (cardinality(@ids::uuid[]) = 0 OR id = ANY (@ids::uuid[]))
ORDER BY created_at, id;

-- name: IsMember :one
SELECT EXISTS (
    SELECT 1 FROM memberships WHERE account_id = @account_id AND user_id = @user_id
) AS member;
