package store_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/auth"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store"
)

var mailDay = time.Date(2026, 10, 10, 6, 0, 0, 0, time.UTC)

const guestAddress = "emma@example.com"

var emma = domain.Guest{Name: "Emma Clarke", Email: guestAddress}

// pageStay is a stay booked on the booking page by a guest with an email, waiting for a bank
// transfer due at due.
func (h house) pageStay(t *testing.T, guest domain.Guest, checkIn, checkOut string, due time.Time) uuid.UUID {
	t.Helper()
	token, hash, err := domain.NewGuestToken()
	if err != nil {
		t.Fatal(err)
	}
	id, err := h.s.CreateGuestBooking(context.Background(), h.t.AccountID, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r1, CheckIn: day(checkIn), CheckOut: day(checkOut), Adults: 2,
			Guest: guest, Source: "page"},
		Status: domain.StatusAwaitingPayment, Currency: "USD", RefPrefix: "TH", Price: domain.Pricing{Total: 24000},
		LedgerUnits: []uuid.UUID{h.r1}, PaymentMethod: "bank", GuestToken: token, GuestTokenHash: hash,
		PaymentDueAt: &due,
	}, mailDay)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

// confirmedStay is a stay the owner entered as confirmed, for a guest with an email.
func (h house) confirmedStay(t *testing.T, checkIn, checkOut string) domain.Booking {
	t.Helper()
	b, err := h.s.CreateBooking(context.Background(), h.t, domain.NewBooking{
		PropertyID: h.p.ID,
		Stay: domain.StayInput{UnitID: h.r2, CheckIn: day(checkIn), CheckOut: day(checkOut), Adults: 2,
			Guest: emma, Source: "whatsapp"},
		Status: domain.StatusConfirmed, Currency: "USD", RefPrefix: "TH", Price: domain.Pricing{Total: 24000},
		LedgerUnits: []uuid.UUID{h.r2},
	}, mailDay)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

type mailRow struct {
	kind, status, to string
	userID           *uuid.UUID
	scheduledFor     time.Time
}

func (h house) mail(t *testing.T) []mailRow {
	t.Helper()
	rows, err := h.s.Pool().Query(context.Background(),
		"SELECT kind, status, to_address, user_id, scheduled_for FROM emails ORDER BY created_at, id")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []mailRow
	for rows.Next() {
		var r mailRow
		if err := rows.Scan(&r.kind, &r.status, &r.to, &r.userID, &r.scheduledFor); err != nil {
			t.Fatal(err)
		}
		out = append(out, r)
	}
	return out
}

func (h house) mailOf(t *testing.T, kind domain.EmailKind) []mailRow {
	t.Helper()
	var out []mailRow
	for _, r := range h.mail(t) {
		if r.kind == string(kind) {
			out = append(out, r)
		}
	}
	return out
}

func kinds(rows []mailRow) string {
	out := make([]string, len(rows))
	for i, r := range rows {
		out[i] = r.kind + " to " + r.to
	}
	return strings.Join(out, ", ")
}

// send runs the send job at now with a sender that always succeeds, and returns what it sent.
func (h house) send(t *testing.T, now time.Time) []store.Email {
	t.Helper()
	var sent []store.Email
	if _, err := h.s.SendEmails(context.Background(), 50, now, func(_ context.Context, e store.Email) (string, error) {
		sent = append(sent, e)
		return "msg-" + e.ID.String(), nil
	}); err != nil {
		t.Fatal(err)
	}
	return sent
}

// confirmedOwner gives the house's owner a confirmed email.
func (h house) confirmedOwner(t *testing.T, address string) {
	t.Helper()
	h.setEmail(t, h.t.UserID, address)
	if _, err := h.s.Pool().Exec(context.Background(), "UPDATE users SET email_confirmed_at = now() WHERE id = $1", h.t.UserID); err != nil {
		t.Fatal(err)
	}
}

func (h house) setEmail(t *testing.T, userID uuid.UUID, address string) {
	t.Helper()
	if _, err := h.s.UpdateUser(context.Background(), userID, domain.UserPatch{Email: &address}, mailDay); err != nil {
		t.Fatal(err)
	}
}

func TestAPageBookingQueuesOneReceivedEmail(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	if got := h.mailOf(t, domain.EmailReceived); len(got) != 1 {
		t.Errorf("received emails = %d, want 1", len(got))
	}
}

func TestAGuestWithoutAnEmailGetsNone(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.pageStay(t, stranger, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	if got := h.mail(t); len(got) != 0 {
		t.Errorf("emails = %s, want none", kinds(got))
	}
}

func TestTheReceivedEmailLinksToAWorkingStayPage(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	id := h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	sent := h.send(t, mailDay)
	_, token, _ := strings.Cut(sent[0].Link, "?t=")
	if _, err := h.s.GuestBooking(context.Background(), id, domain.GuestTokenHash(token)); err != nil {
		t.Errorf("stay page with the emailed token: %v", err)
	}
}

func TestAnOwnerStayGetsATokenWhenAnEmailNeedsOne(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	b := h.confirmedStay(t, "2026-10-20", "2026-10-22")
	sent := h.send(t, mailDay)
	_, token, _ := strings.Cut(sent[0].Link, "?t=")
	if _, err := h.s.GuestBooking(context.Background(), b.ID, domain.GuestTokenHash(token)); err != nil {
		t.Errorf("stay page with the emailed token: %v", err)
	}
}

func TestConfirmingTwiceQueuesOneConfirmedEmail(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	id := h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	for range 2 {
		if _, err := h.s.RecordPayment(ctx, h.t, id, domain.Payment{Method: "bank", Amount: 12000, ReceivedAt: mailDay}); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.mailOf(t, domain.EmailConfirmed); len(got) != 1 {
		t.Errorf("confirmed emails = %d, want 1", len(got))
	}
}

func TestADeclinedRequestEmailsTheGuest(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	b, err := h.s.CreateBooking(ctx, h.t, domain.NewBooking{
		PropertyID: h.p.ID, Stay: domain.StayInput{UnitID: h.r1, CheckIn: day("2026-10-20"), CheckOut: day("2026-10-22"),
			Adults: 1, Guest: emma, Source: "page"},
		Status: domain.StatusRequested, Currency: "USD", RefPrefix: "TH", Price: domain.Pricing{Total: 24000},
		LedgerUnits: []uuid.UUID{h.r1},
	}, mailDay)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.TransitionBooking(ctx, h.t, b.ID, b.Version, domain.StatusDeclined, "", mailDay); err != nil {
		t.Fatal(err)
	}
	if got := h.mailOf(t, domain.EmailDeclined); len(got) != 1 {
		t.Errorf("declined emails = %d, want 1", len(got))
	}
}

func TestACancelledStaysReminderIsSkipped(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	due := mailDay.Add(30 * time.Hour)
	id := h.pageStay(t, emma, "2026-11-20", "2026-11-22", due)
	if _, err := h.s.QueueTimedGuestEmails(ctx, mailDay.Add(6*time.Hour)); err != nil {
		t.Fatal(err)
	}
	b, err := h.s.GetBooking(ctx, h.t, id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.CancelBooking(ctx, h.t, id, b.Version, "Guest changed plans", nil, mailDay.Add(7*time.Hour)); err != nil {
		t.Fatal(err)
	}
	h.send(t, due.Add(-23*time.Hour))
	if got := h.mailOf(t, domain.EmailReminder); len(got) != 1 || got[0].status != "skipped" {
		t.Errorf("reminders = %+v, want one skipped", got)
	}
}

func TestThePreArrivalIsQueuedAt0900TwoDaysBefore(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedStay(t, "2026-10-22", "2026-10-25")
	for range 2 {
		if _, err := h.s.QueueTimedGuestEmails(context.Background(), time.Date(2026, 10, 19, 10, 0, 0, 0, time.UTC)); err != nil {
			t.Fatal(err)
		}
	}
	want := time.Date(2026, 10, 20, 3, 30, 0, 0, time.UTC) // 09:00 in Colombo
	if got := h.mailOf(t, domain.EmailPrearrival); len(got) != 1 || !got[0].scheduledFor.Equal(want) {
		t.Errorf("pre-arrival emails = %+v, want one at %s", got, want)
	}
}

func TestATemplateWithEmailOffIsSkipped(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	templates, err := h.s.ListTemplates(ctx, h.t)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.UpdateTemplate(ctx, h.t, templates[0].ID, nil, []string{"whatsapp"}); err != nil {
		t.Fatal(err)
	}
	h.confirmedStay(t, "2026-10-20", "2026-10-22")
	h.send(t, mailDay)
	if got := h.mailOf(t, domain.EmailConfirmed); len(got) != 1 || got[0].status != "skipped" {
		t.Errorf("confirmed emails = %+v, want one skipped", got)
	}
}

func TestASuppressedAddressIsNotSent(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	if err := h.s.RecordSuppression(context.Background(), "Emma@Example.com", "bounce"); err != nil {
		t.Fatal(err)
	}
	h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	h.send(t, mailDay)
	if got := h.mailOf(t, domain.EmailReceived); len(got) != 1 || got[0].status != "suppressed" {
		t.Errorf("received emails = %+v, want one suppressed", got)
	}
}

func TestASuppressedAddressLosesItsConfirmation(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.confirmedOwner(t, "owner@example.com")
	if err := h.s.RecordSuppression(ctx, "owner@example.com", "complaint"); err != nil {
		t.Fatal(err)
	}
	if u, err := h.s.GetUser(ctx, h.t.UserID); err != nil || u.EmailConfirmed {
		t.Errorf("confirmed after suppression = %v (err %v), want false", u.EmailConfirmed, err)
	}
}

func TestAClosedAccountSendsNothing(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	if _, err := h.s.DeleteUser(context.Background(), h.t.UserID, mailDay); err != nil {
		t.Fatal(err)
	}
	if sent := h.send(t, mailDay); len(sent) != 0 {
		t.Errorf("sent %d emails for a closed account, want none", len(sent))
	}
}

func TestAFailedSendIsTriedThreeTimes(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.pageStay(t, emma, "2026-10-20", "2026-10-22", mailDay.Add(48*time.Hour))
	fail := func(context.Context, store.Email) (string, error) { return "", errors.New("ses down") }
	for i := range 4 {
		if _, err := h.s.SendEmails(context.Background(), 50, mailDay.Add(time.Duration(i)*time.Hour), fail); err != nil {
			t.Fatal(err)
		}
	}
	if got := h.mailOf(t, domain.EmailReceived); got[0].status != "failed" {
		t.Errorf("status after three failures = %s, want failed", got[0].status)
	}
}

func TestTheFirstEmailGetsTheWelcome(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.setEmail(t, h.t.UserID, "owner@example.com")
	if got := kinds(h.mail(t)); got != "welcome to owner@example.com" {
		t.Errorf("emails = %s, want the welcome", got)
	}
}

func TestAChangedEmailIsConfirmedAndTheOldOneTold(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "old@example.com")
	h.setEmail(t, h.t.UserID, "new@example.com")
	want := "welcome to old@example.com, email_changed to old@example.com, confirm_email to new@example.com"
	if got := kinds(h.mail(t)); got != want {
		t.Errorf("emails = %s, want %s", got, want)
	}
}

func TestAChangedEmailIsNoLongerConfirmed(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "old@example.com")
	h.setEmail(t, h.t.UserID, "new@example.com")
	if u, err := h.s.GetUser(context.Background(), h.t.UserID); err != nil || u.EmailConfirmed {
		t.Errorf("confirmed = %v (err %v), want false", u.EmailConfirmed, err)
	}
}

// confirmationToken sends the queued emails and returns the token in the confirmation link.
func (h house) confirmationToken(t *testing.T) string {
	t.Helper()
	for _, e := range h.send(t, mailDay) {
		if e.Kind.CarriesConfirmation() {
			return e.Link[strings.LastIndex(e.Link, "/")+1:]
		}
	}
	t.Fatal("no confirmation email was sent")
	return ""
}

func TestTheConfirmationLinkConfirmsTheEmail(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.setEmail(t, h.t.UserID, "owner@example.com")
	if _, err := h.s.ConfirmEmail(ctx, h.confirmationToken(t), mailDay); err != nil {
		t.Fatal(err)
	}
	if u, err := h.s.GetUser(ctx, h.t.UserID); err != nil || !u.EmailConfirmed {
		t.Errorf("confirmed = %v (err %v), want true", u.EmailConfirmed, err)
	}
}

func TestAUsedConfirmationTokenIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.setEmail(t, h.t.UserID, "owner@example.com")
	token := h.confirmationToken(t)
	if _, err := h.s.ConfirmEmail(ctx, token, mailDay); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.ConfirmEmail(ctx, token, mailDay); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("second use = %v, want not found", err)
	}
}

func TestAnExpiredConfirmationTokenIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.setEmail(t, h.t.UserID, "owner@example.com")
	token := h.confirmationToken(t)
	if _, err := h.s.ConfirmEmail(context.Background(), token, mailDay.Add(8*24*time.Hour)); !errors.Is(err, domain.ErrNotFound) {
		t.Errorf("use after 8 days = %v, want not found", err)
	}
}

func TestResendingAConfirmedEmailIsRefused(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	if err := h.s.ResendConfirmation(context.Background(), h.t.UserID, mailDay); !errors.Is(err, domain.ErrEmailConfirmed) {
		t.Errorf("resend = %v, want ErrEmailConfirmed", err)
	}
}

func (h house) signInOn(t *testing.T, device string) {
	t.Helper()
	ctx := context.Background()
	if err := h.s.NoteSignIn(ctx, h.t.UserID, device, mailDay); err != nil {
		t.Fatal(err)
	}
	if err := h.s.CreateSession(ctx, auth.Session{
		ID: domain.NewID(), UserID: h.t.UserID, RefreshHash: []byte(domain.NewID().String()), DeviceName: device,
		ExpiresAt: mailDay.Add(time.Hour),
	}, mailDay); err != nil {
		t.Fatal(err)
	}
}

func TestTheFirstSignInSendsNoNotice(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	h.signInOn(t, "Nimal's iPhone")
	if got := h.mailOf(t, domain.EmailNewSignIn); len(got) != 0 {
		t.Errorf("new sign-in emails = %d, want 0", len(got))
	}
}

func TestASecondDeviceSendsANotice(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	h.signInOn(t, "Nimal's iPhone")
	h.signInOn(t, "Pixel 9")
	if got := h.mailOf(t, domain.EmailNewSignIn); len(got) != 1 {
		t.Errorf("new sign-in emails = %d, want 1", len(got))
	}
}

func TestTheSameDeviceSendsNoNotice(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	h.signInOn(t, "Nimal's iPhone")
	h.signInOn(t, "Nimal's iPhone")
	if got := h.mailOf(t, domain.EmailNewSignIn); len(got) != 0 {
		t.Errorf("new sign-in emails = %d, want 0", len(got))
	}
}

func TestDeletingQueuesTheScheduledNotice(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	if _, err := h.s.DeleteUser(context.Background(), h.t.UserID, mailDay); err != nil {
		t.Fatal(err)
	}
	if got := h.mailOf(t, domain.EmailDeletionScheduled); len(got) != 1 {
		t.Errorf("deletion emails = %d, want 1", len(got))
	}
}

func TestRestoringQueuesTheRestoredNotice(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.confirmedOwner(t, "owner@example.com")
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, mailDay); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.RestoreUser(ctx, h.t.UserID, mailDay.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := h.mailOf(t, domain.EmailAccountRestored); len(got) != 1 {
		t.Errorf("restored emails = %d, want 1", len(got))
	}
}

func TestThePurgeEmailOutlivesThePerson(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	ctx := context.Background()
	h.confirmedOwner(t, "owner@example.com")
	if _, err := h.s.DeleteUser(ctx, h.t.UserID, mailDay); err != nil {
		t.Fatal(err)
	}
	if _, err := h.s.PurgeDeletedUsers(ctx, mailDay.AddDate(0, 0, 31)); err != nil {
		t.Fatal(err)
	}
	if got := h.mailOf(t, domain.EmailDeletionCompleted); len(got) != 1 || got[0].to != "owner@example.com" || got[0].userID != nil {
		t.Errorf("completed emails = %+v, want one to the captured address", got)
	}
}

func TestAPaidWebPeriodQueuesOneReceipt(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.confirmedOwner(t, "owner@example.com")
	h.payWeb(context.Background(), t, "evt-1", mailDay)
	if got := h.mailOf(t, domain.EmailPlanReceipt); len(got) != 1 {
		t.Errorf("receipts = %d, want 1", len(got))
	}
}

func TestAnUnconfirmedOwnerGetsNoReceipt(t *testing.T) {
	t.Parallel()
	h := newHouse(t)
	h.setEmail(t, h.t.UserID, "owner@example.com")
	h.payWeb(context.Background(), t, "evt-1", mailDay)
	if got := h.mailOf(t, domain.EmailPlanReceipt); len(got) != 0 {
		t.Errorf("receipts = %d, want 0", len(got))
	}
}
