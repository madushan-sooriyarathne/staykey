package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
	"staykey.direct/api/internal/tenant"
)

// StayKey payouts: card money StayKey collects for an owner, kept as a ledger in rupee cents and
// paid out by bank transfer every other Monday. Jobs and the payouts command call the methods
// without a tenant; owner routes pass one.

// ErrBatchPaid means a payout batch was already marked paid, or doesn't exist.
var ErrBatchPaid = errors.New("that batch doesn't exist or is already paid")

// ErrHeldPayouts means a batch still has payouts the export left out because their bank details
// changed; fail them before marking the batch paid.
var ErrHeldPayouts = errors.New("the batch has held payouts the export left out; fail them first")

// ErrPayoutClosed means a payout doesn't exist or has already failed.
var ErrPayoutClosed = errors.New("that payout doesn't exist or has already failed")

func (s *Postgres) payoutFeeBPS() int {
	if s.cards.PayoutFeeBPS > 0 {
		return s.cards.PayoutFeeBPS
	}
	return domain.DefaultPayoutFeeBPS
}

// payoutProfileComplete reports whether the account has given its payout details under the
// current terms.
func (s *Postgres) payoutProfileComplete(ctx context.Context, accountID uuid.UUID) (bool, error) {
	p, err := s.db(ctx).GetPayoutProfile(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("payout profile: %w", err)
	}
	return p.TermsVersion == domain.PayoutTermsVersion, nil
}

// PayoutProfile returns the account's payout details, with sealed numbers by their last four.
func (s *Postgres) PayoutProfile(ctx context.Context, accountID uuid.UUID) (domain.PayoutProfile, error) {
	p, err := s.db(ctx).GetPayoutProfile(ctx, accountID)
	if err != nil {
		return domain.PayoutProfile{}, notFound(err)
	}
	return payoutProfile(p), nil
}

func payoutProfile(p queries.GetPayoutProfileRow) domain.PayoutProfile {
	return domain.PayoutProfile{
		LegalName: p.LegalName, NICLast4: p.NicLast4, BankName: p.BankName, Branch: p.Branch,
		AccountName: p.AccountName, AccountLast4: p.AccountLast4, TermsVersion: p.TermsVersion,
		AcceptedAt: p.AcceptedAt, BankChangedAt: p.BankChangedAt,
	}
}

func nicContext(accountID uuid.UUID) []byte { return []byte("payout-nic:" + accountID.String()) }

func payoutAccountContext(accountID uuid.UUID) []byte {
	return []byte("payout-account:" + accountID.String())
}

// SetPayoutProfile saves the owner's payout details. New details, or a new bank, branch or
// account number, hold payouts for 72 hours; a change also tells the owner by email and in
// Activity, in case it wasn't them.
func (s *Postgres) SetPayoutProfile(ctx context.Context, t tenant.Tenant, in domain.PayoutProfileInput, now time.Time) (domain.PayoutProfile, error) {
	in.Normalize()
	if err := in.Validate(); err != nil {
		return domain.PayoutProfile{}, err
	}
	if s.secrets == nil {
		return domain.PayoutProfile{}, errNoSecrets
	}
	nic, err := s.secrets.Seal([]byte(in.NIC), nicContext(t.AccountID))
	if err != nil {
		return domain.PayoutProfile{}, fmt.Errorf("seal nic: %w", err)
	}
	number, err := s.secrets.Seal([]byte(in.AccountNumber), payoutAccountContext(t.AccountID))
	if err != nil {
		return domain.PayoutProfile{}, fmt.Errorf("seal account number: %w", err)
	}
	err = s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		bankChangedAt, changed := now, false
		old, err := q.GetPayoutProfile(ctx, t.AccountID)
		switch {
		case err == nil:
			oldNumber, err := s.secrets.Open(old.AccountNumberSealed, payoutAccountContext(t.AccountID))
			if err != nil {
				return fmt.Errorf("open account number: %w", err)
			}
			changed = string(oldNumber) != in.AccountNumber || old.BankName != in.BankName || old.Branch != in.Branch
			if !changed {
				bankChangedAt = old.BankChangedAt
			}
		case !errors.Is(err, pgx.ErrNoRows):
			return fmt.Errorf("payout profile: %w", err)
		}
		if err := q.UpsertPayoutProfile(ctx, queries.UpsertPayoutProfileParams{
			AccountID: t.AccountID, LegalName: in.LegalName, NicSealed: nic, NicLast4: domain.Last4(in.NIC),
			BankName: in.BankName, Branch: in.Branch, AccountName: in.AccountName, AccountNumberSealed: number,
			AccountLast4: domain.Last4(in.AccountNumber), TermsVersion: in.AcceptTerms, AcceptedAt: now,
			BankChangedAt: bankChangedAt,
		}); err != nil {
			return fmt.Errorf("save payout profile: %w", err)
		}
		if !changed {
			return nil
		}
		heldUntil := now.Add(domain.PayoutHold)
		if err := s.addActivity(ctx, t.AccountID, domain.PayoutBankChanged(heldUntil), &t.UserID, now); err != nil {
			return err
		}
		return s.queueOwnerPayoutEmail(ctx, t.AccountID, domain.EmailPayoutBankChanged, domain.PayoutNotice{
			BankName: in.BankName, AccountLast4: domain.Last4(in.AccountNumber), HeldUntil: heldUntil,
		}, "", now)
	})
	if err != nil {
		return domain.PayoutProfile{}, err
	}
	return s.PayoutProfile(ctx, t.AccountID)
}

// queueOwnerPayoutEmail emails the account's owner about their payouts, if their address is
// confirmed.
func (s *Postgres) queueOwnerPayoutEmail(ctx context.Context, accountID uuid.UUID, kind domain.EmailKind, n domain.PayoutNotice, dedupeKey string, now time.Time) error {
	owner, err := s.db(ctx).OwnerConfirmedEmail(ctx, accountID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("owner email: %w", err)
	}
	return s.queueEmail(ctx, queuedEmail{
		accountID: &accountID, userID: &owner.ID, kind: kind, to: owner.Email,
		payload: emailPayload{Name: owner.Name, Payout: &n}, dedupeKey: dedupeKey, at: now,
	})
}

// cardPaymentCollected enters a card payment StayKey collected in the owner's payout ledger:
// the rupees charged, less StayKey's processing charge, both payable the day after check-in.
func (s *Postgres) cardPaymentCollected(ctx context.Context, c queries.CardCheckout, paymentID uuid.UUID, b domain.Booking, now time.Time) error {
	if domain.Collector(c.CollectedBy) != domain.CollectedByStayKey {
		return nil
	}
	available := domain.AvailableOn(b.CheckIn)
	entries := []struct {
		kind   domain.PayoutEntryKind
		amount int64
	}{
		{domain.EntryPayment, c.ChargedCents},
		{domain.EntryProcessing, -domain.ProcessingCharge(c.ChargedCents, s.payoutFeeBPS())},
	}
	for _, e := range entries {
		if e.amount == 0 {
			continue
		}
		if err := s.addPayoutEntry(ctx, queries.InsertPayoutEntryParams{
			AccountID: c.AccountID, BookingID: &b.ID, PaymentID: &paymentID, Kind: string(e.kind),
			Amount: e.amount, AvailableOn: available, Note: b.Ref, CreatedAt: now,
		}); err != nil {
			return err
		}
	}
	return nil
}

// cardRefundSent takes a card refund of money StayKey collected out of the owner's payout
// ledger, once, when Payments.lk accepts or reports it. The processing charge stays.
func (s *Postgres) cardRefundSent(ctx context.Context, refundID uuid.UUID, now time.Time) error {
	r, err := s.db(ctx).CollectedRefund(ctx, refundID)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("collected refund: %w", err)
	}
	return s.addPayoutEntry(ctx, queries.InsertPayoutEntryParams{
		AccountID: r.AccountID, BookingID: &r.BookingID, PaymentID: &refundID, Kind: string(domain.EntryRefund),
		Amount: -deref(r.ChargedCents), AvailableOn: domain.PayoutDay(now), Note: "Card refund", CreatedAt: now,
	})
}

// cardRefundReturned puts a refund that failed back into the owner's payout ledger, if it was
// taken out.
func (s *Postgres) cardRefundReturned(ctx context.Context, _, refundID uuid.UUID, now time.Time) error {
	if _, err := s.db(ctx).ReturnRefundEntry(ctx, queries.ReturnRefundEntryParams{
		ID: domain.NewID(), AvailableOn: domain.PayoutDay(now), CreatedAt: now, PaymentID: &refundID,
	}); err != nil {
		return fmt.Errorf("return refund entry: %w", err)
	}
	return nil
}

func (s *Postgres) addPayoutEntry(ctx context.Context, e queries.InsertPayoutEntryParams) error {
	if e.ID == uuid.Nil {
		e.ID = domain.NewID()
	}
	if _, err := s.db(ctx).InsertPayoutEntry(ctx, e); err != nil {
		return fmt.Errorf("payout entry: %w", err)
	}
	return nil
}

// nextPayoutDay is the next payout Monday whose batch isn't prepared yet.
func (s *Postgres) nextPayoutDay(ctx context.Context, now time.Time) (time.Time, error) {
	day := domain.NextPayoutDay(domain.PayoutDay(now))
	done, err := s.db(ctx).HasPayoutBatch(ctx, domain.PeriodEnd(day))
	if err != nil {
		return time.Time{}, fmt.Errorf("payout batch: %w", err)
	}
	if done {
		day = day.AddDate(0, 0, domain.PayoutEveryDays)
	}
	return day, nil
}

// Payouts returns the account's balances, next payout day and past payouts.
func (s *Postgres) Payouts(ctx context.Context, accountID uuid.UUID, now time.Time) (domain.Payouts, error) {
	q := s.db(ctx)
	next, err := s.nextPayoutDay(ctx, now)
	if err != nil {
		return domain.Payouts{}, err
	}
	bal, err := q.PayoutBalance(ctx, queries.PayoutBalanceParams{PayableOn: domain.PeriodEnd(next), AccountID: accountID})
	if err != nil {
		return domain.Payouts{}, fmt.Errorf("payout balance: %w", err)
	}
	out := domain.Payouts{Available: bal.Available, Pending: bal.Pending, OnTheWay: bal.OnTheWay, NextPayoutDay: next}
	p, err := q.GetPayoutProfile(ctx, accountID)
	switch {
	case err == nil:
		out.HeldUntil = payoutProfile(p).HeldUntil(now)
	case !errors.Is(err, pgx.ErrNoRows):
		return domain.Payouts{}, fmt.Errorf("payout profile: %w", err)
	}
	rows, err := q.ListPayouts(ctx, queries.ListPayoutsParams{AccountID: accountID, RowLimit: 100})
	if err != nil {
		return domain.Payouts{}, fmt.Errorf("list payouts: %w", err)
	}
	out.History = make([]domain.Payout, len(rows))
	for i, r := range rows {
		out.History[i] = payout(queries.GetPayoutRow(r))
	}
	return out, nil
}

func payout(r queries.GetPayoutRow) domain.Payout {
	return domain.Payout{
		ID: r.ID, PeriodEnd: r.PeriodEnd, Amount: r.Amount, Status: domain.PayoutStatus(r.Status),
		Reason: deref(r.Reason), PreparedAt: r.PreparedAt, PaidAt: r.PaidAt, FailedAt: r.FailedAt,
	}
}

// PayoutStatement returns one of the account's payouts with the entries it paid.
func (s *Postgres) PayoutStatement(ctx context.Context, accountID, payoutID uuid.UUID) (domain.PayoutStatement, error) {
	q := s.db(ctx)
	r, err := q.GetPayout(ctx, queries.GetPayoutParams{ID: payoutID, AccountID: accountID})
	if err != nil {
		return domain.PayoutStatement{}, notFound(err)
	}
	rows, err := q.PayoutStatement(ctx, queries.PayoutStatementParams{PayoutID: &payoutID, AccountID: accountID})
	if err != nil {
		return domain.PayoutStatement{}, fmt.Errorf("payout statement: %w", err)
	}
	out := domain.PayoutStatement{Payout: payout(r), Lines: make([]domain.StatementLine, len(rows))}
	for i, l := range rows {
		out.Lines[i] = domain.StatementLine{
			ID: l.ID, Kind: domain.PayoutEntryKind(l.Kind), Amount: l.Amount, AvailableOn: l.AvailableOn,
			Note: l.Note, BookingID: l.BookingID, BookingRef: deref(l.BookingRef), GuestName: deref(l.GuestName),
			CheckIn: l.CheckIn,
		}
	}
	return out, nil
}

// PreparedBatch is what PreparePayouts did.
type PreparedBatch struct {
	ID        uuid.UUID
	PeriodEnd time.Time
	Payouts   int
	Total     int64
}

// PreparePayouts prepares the batch of the latest payout Monday, once: a payout for each open
// account owed at least the minimum by the Sunday before, whose bank details haven't changed in
// the last 72 hours. It reports false when there is no batch to prepare.
func (s *Postgres) PreparePayouts(ctx context.Context, now time.Time) (PreparedBatch, bool, error) {
	day, ok := domain.LatestPayoutDay(domain.PayoutDay(now))
	if !ok {
		return PreparedBatch{}, false, nil
	}
	out := PreparedBatch{PeriodEnd: domain.PeriodEnd(day)}
	prepared := false
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		id, err := q.InsertPayoutBatch(ctx, queries.InsertPayoutBatchParams{ID: domain.NewID(), PeriodEnd: out.PeriodEnd, PreparedAt: now})
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("insert payout batch: %w", err)
		}
		out.ID, prepared = id, true
		owed, err := q.PayoutCandidates(ctx, queries.PayoutCandidatesParams{
			PeriodEnd: out.PeriodEnd, HeldBefore: now.Add(-domain.PayoutHold), Minimum: domain.PayoutMinimum,
		})
		if err != nil {
			return fmt.Errorf("payout candidates: %w", err)
		}
		for _, o := range owed {
			amount, err := s.preparePayout(ctx, id, o.AccountID, o.Amount, out.PeriodEnd, now)
			if err != nil {
				return err
			}
			out.Payouts++
			out.Total += amount
		}
		return nil
	})
	return out, prepared, err
}

func (s *Postgres) preparePayout(ctx context.Context, batchID, accountID uuid.UUID, amount int64, periodEnd, now time.Time) (int64, error) {
	q := s.db(ctx)
	id := domain.NewID()
	if err := q.InsertPayout(ctx, queries.InsertPayoutParams{
		ID: id, AccountID: accountID, BatchID: batchID, PeriodEnd: periodEnd, Amount: amount, PreparedAt: now,
	}); err != nil {
		return 0, fmt.Errorf("insert payout: %w", err)
	}
	attached, err := q.AttachPayoutEntries(ctx, queries.AttachPayoutEntriesParams{PayoutID: &id, AccountID: accountID, PeriodEnd: periodEnd})
	if err != nil {
		return 0, fmt.Errorf("attach payout entries: %w", err)
	}
	if attached != amount {
		// An entry arrived between reading the balance and attaching; pay what was attached.
		if attached <= 0 {
			return 0, fmt.Errorf("payout for account %s attached %d", accountID, attached)
		}
		if err := q.SetPayoutAmount(ctx, queries.SetPayoutAmountParams{ID: id, Amount: attached}); err != nil {
			return 0, fmt.Errorf("set payout amount: %w", err)
		}
	}
	return attached, s.addPayoutEntry(ctx, queries.InsertPayoutEntryParams{
		AccountID: accountID, Kind: string(domain.EntryPayout), Amount: -attached, AvailableOn: periodEnd,
		PayoutID: &id, CreatedAt: now,
	})
}

// PayoutBatch is a batch as the payouts command lists it.
type PayoutBatch struct {
	ID         uuid.UUID
	PeriodEnd  time.Time
	PreparedAt time.Time
	PaidAt     *time.Time
	BankRef    string
	Payouts    int64
	Total      int64
}

// PayoutBatches lists the latest batches, newest first.
func (s *Postgres) PayoutBatches(ctx context.Context, limit int) ([]PayoutBatch, error) {
	rows, err := s.db(ctx).ListPayoutBatches(ctx, int32(limit))
	if err != nil {
		return nil, fmt.Errorf("list payout batches: %w", err)
	}
	out := make([]PayoutBatch, len(rows))
	for i, r := range rows {
		out[i] = PayoutBatch{
			ID: r.ID, PeriodEnd: r.PeriodEnd, PreparedAt: r.PreparedAt, PaidAt: r.PaidAt, BankRef: deref(r.BankRef),
			Payouts: r.Payouts, Total: r.Total,
		}
	}
	return out, nil
}

// PayoutTransfer is one bank transfer of a batch. Held is set when the bank details changed
// after the payout was prepared: it must not be sent to the old or the new account without a
// check, so the operator fails it and the money waits for the next payout.
type PayoutTransfer struct {
	PayoutID      uuid.UUID
	AccountID     uuid.UUID
	Amount        int64
	Status        domain.PayoutStatus
	AccountName   string
	BankName      string
	Branch        string
	AccountNumber string
	Held          bool
}

// BatchTransfers lists a batch's payouts with the bank details to pay them to.
func (s *Postgres) BatchTransfers(ctx context.Context, batchID uuid.UUID) ([]PayoutTransfer, error) {
	if _, err := s.db(ctx).GetPayoutBatch(ctx, batchID); err != nil {
		return nil, notFound(err)
	}
	if s.secrets == nil {
		return nil, errNoSecrets
	}
	rows, err := s.db(ctx).BatchPayouts(ctx, batchID)
	if err != nil {
		return nil, fmt.Errorf("batch payouts: %w", err)
	}
	out := make([]PayoutTransfer, len(rows))
	for i, r := range rows {
		number, err := s.secrets.Open(r.AccountNumberSealed, payoutAccountContext(r.AccountID))
		if err != nil {
			return nil, fmt.Errorf("open account number of account %s: %w", r.AccountID, err)
		}
		out[i] = PayoutTransfer{
			PayoutID: r.ID, AccountID: r.AccountID, Amount: r.Amount, Status: domain.PayoutStatus(r.Status),
			AccountName: r.AccountName, BankName: r.BankName, Branch: r.Branch, AccountNumber: string(number),
			Held: r.BankChangedAt.After(r.PreparedAt),
		}
	}
	return out, nil
}

// MarkBatchPaid records that StayKey's team paid a batch, marks its prepared payouts paid, and
// tells each owner. A batch with held payouts still prepared is refused. It returns how many payouts it marked.
func (s *Postgres) MarkBatchPaid(ctx context.Context, batchID uuid.UUID, bankRef string, now time.Time) (int, error) {
	n := 0
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		held, err := q.HeldBatchPayouts(ctx, batchID)
		if err != nil {
			return fmt.Errorf("held payouts: %w", err)
		}
		if held > 0 {
			return ErrHeldPayouts
		}
		rows, err := q.MarkBatchPaid(ctx, queries.MarkBatchPaidParams{ID: batchID, PaidAt: &now, BankRef: &bankRef})
		if err != nil {
			return fmt.Errorf("mark batch paid: %w", err)
		}
		if rows == 0 {
			return ErrBatchPaid
		}
		paid, err := q.MarkBatchPayoutsPaid(ctx, queries.MarkBatchPayoutsPaidParams{BatchID: batchID, PaidAt: &now})
		if err != nil {
			return fmt.Errorf("mark payouts paid: %w", err)
		}
		for _, p := range paid {
			if err := s.addActivity(ctx, p.AccountID, domain.PayoutSent(p.Amount, p.PeriodEnd), nil, now); err != nil {
				return err
			}
			if err := s.queueOwnerPayoutEmail(ctx, p.AccountID, domain.EmailPayoutSent, domain.PayoutNotice{
				Amount: p.Amount, PeriodEnd: p.PeriodEnd,
			}, p.ID.String()+":payout_sent", now); err != nil {
				return err
			}
		}
		n = len(paid)
		return nil
	})
	return n, err
}

// FailPayout records a payout that didn't reach the owner, whether it was never sent or the
// bank sent it back, and puts its money back in the owner's balance for the next payout.
func (s *Postgres) FailPayout(ctx context.Context, payoutID uuid.UUID, reason string, now time.Time) (int64, error) {
	var amount int64
	err := s.InTx(ctx, func(ctx context.Context) error {
		q := s.db(ctx)
		r, err := q.FailPayout(ctx, queries.FailPayoutParams{ID: payoutID, Reason: &reason, FailedAt: &now})
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrPayoutClosed
		}
		if err != nil {
			return fmt.Errorf("fail payout: %w", err)
		}
		amount = r.Amount
		if err := s.addPayoutEntry(ctx, queries.InsertPayoutEntryParams{
			AccountID: r.AccountID, Kind: string(domain.EntryPayoutReturned), Amount: r.Amount,
			AvailableOn: domain.PayoutDay(now), Note: "Payout " + r.ID.String() + " returned: " + truncate(reason, 200), CreatedAt: now,
		}); err != nil {
			return err
		}
		return s.addActivity(ctx, r.AccountID, domain.PayoutReturned(r.Amount), nil, now)
	})
	return amount, err
}

// AdjustPayouts adds a correction to an account's payout ledger, payable at once: a chargeback
// as a negative amount, money owed to the owner as a positive one.
func (s *Postgres) AdjustPayouts(ctx context.Context, accountID uuid.UUID, amount int64, reason string, now time.Time) error {
	if amount == 0 {
		return &domain.ValidationError{Field: "amount", Message: "must not be zero"}
	}
	if reason == "" {
		return &domain.ValidationError{Field: "reason", Message: "is required"}
	}
	return s.addPayoutEntry(ctx, queries.InsertPayoutEntryParams{
		AccountID: accountID, Kind: string(domain.EntryAdjustment), Amount: amount,
		AvailableOn: domain.PayoutDay(now), Note: truncate(reason, 300), CreatedAt: now,
	})
}
