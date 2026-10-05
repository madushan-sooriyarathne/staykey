package store

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// IdempotencyTTL is how long a key's reply is kept for retries.
const IdempotencyTTL = 24 * time.Hour

// Idempotent runs fn once per key and request. fn runs in the same transaction as the key's
// row, so a retry that arrives while the first attempt runs waits for it, then gets its reply.
// A failed fn leaves no trace and can be retried. The reply is fn's body, or the stored one.
func (s *Postgres) Idempotent(ctx context.Context, t tenant.Tenant, key string, requestHash []byte, status int, now time.Time, fn func(ctx context.Context) ([]byte, error)) ([]byte, error) {
	var reply []byte
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		claim := queries.ClaimIdempotencyKeyParams{AccountID: t.AccountID, Key: key, RequestHash: requestHash, ExpiresAt: now.Add(IdempotencyTTL)}
		claimed, err := q.ClaimIdempotencyKey(ctx, claim)
		if err != nil {
			return fmt.Errorf("claim idempotency key: %w", err)
		}
		if claimed == 0 {
			prev, err := q.GetIdempotencyKey(ctx, queries.GetIdempotencyKeyParams{AccountID: t.AccountID, Key: key})
			if err != nil {
				return fmt.Errorf("read idempotency key: %w", err)
			}
			if prev.ExpiresAt.After(now) {
				if !bytes.Equal(prev.RequestHash, requestHash) {
					return domain.ErrKeyReused
				}
				reply = prev.Response
				return nil
			}
			if err := q.DeleteIdempotencyKey(ctx, queries.DeleteIdempotencyKeyParams{AccountID: t.AccountID, Key: key}); err != nil {
				return fmt.Errorf("drop expired idempotency key: %w", err)
			}
			if _, err := q.ClaimIdempotencyKey(ctx, claim); err != nil {
				return fmt.Errorf("claim idempotency key: %w", err)
			}
		}
		if reply, err = fn(ctx); err != nil {
			return err
		}
		code := int16(status)
		if err := q.SaveIdempotentResponse(ctx, queries.SaveIdempotentResponseParams{
			AccountID: t.AccountID, Key: key, Status: &code, Response: reply,
		}); err != nil {
			return fmt.Errorf("save idempotent response: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return reply, nil
}
