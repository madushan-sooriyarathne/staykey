// Command seed creates the sample account from the owner app's prototype (apps/mobile/src/data/
// seed.ts) on the server: an owner, Kingfisher Villa, Coral Bay House and the team. Development
// only; it refuses to run when APP_ENV=production.
//
// Usage:
//
//	go run ./cmd/seed                          # owner +94770000001
//	go run ./cmd/seed -owner-phone "+94 77 123 4567" -owner-name "Nimal Perera"
//	go run ./cmd/seed -reset                   # delete the sample account first
//
// Sign in on the app with the owner's number, or with the manager or caretaker numbers printed
// at the end, to see the account from each role. Development servers show the code on screen.
//
// It covers what the API holds so far: the account, people and memberships, and both properties
// with their full setup (units, a linked whole-house unit, seasons, discounts, charges, extras,
// promo codes, payments and calendar links), mirroring withSampleSettings in seed.ts. Sample
// bookings join when bookings move to the server in phase 3.
package main

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"math"
	"os"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/config"
	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/secret"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/tenant"
)

type member struct {
	name, phone string
	role        domain.Role
	villaOnly   bool // limited to Kingfisher Villa
}

// The team from sampleTeam in seed.ts.
var team = []member{
	{name: "Ruwan Silva", phone: "+94772223344", role: domain.RoleManager},
	{name: "Kamal Jayasinghe", phone: "+94713338899", role: domain.RoleCaretaker, villaOnly: true},
}

// pendingInvite is the caretaker who has not accepted yet.
var pendingInvite = member{phone: "+94715550199", role: domain.RoleCaretaker, villaOnly: true}

func main() {
	ownerPhone := flag.String("owner-phone", "+94770000001", "phone number the owner signs in with")
	ownerName := flag.String("owner-name", "Amaya Perera", "owner's name")
	reset := flag.Bool("reset", false, "delete the sample account first")
	flag.Parse()

	if err := run(*ownerPhone, *ownerName, *reset); err != nil {
		slog.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(ownerPhone, ownerName string, reset bool) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.IsProduction() {
		return errors.New("seed creates demo data and only runs outside production")
	}
	ownerPhone, err = domain.NormalizePhone(ownerPhone)
	if err != nil {
		return fmt.Errorf("owner phone: %w", err)
	}

	ctx := context.Background()
	box, err := secret.New(cfg.DataKey)
	if err != nil {
		return err
	}
	db, err := store.NewPostgres(ctx, cfg.DatabaseURL, store.WithSecrets(box))
	if err != nil {
		return err
	}
	defer db.Close()

	var summary []string
	err = db.InTx(ctx, func(ctx context.Context) error {
		owner, _, err := db.UpsertUserByPhone(ctx, ownerPhone)
		if err != nil {
			return err
		}
		if _, err := db.UpdateUser(ctx, owner.ID, domain.UserPatch{Name: &ownerName}); err != nil {
			return err
		}

		done, err := clearPrevious(ctx, db, owner.ID, reset)
		if err != nil || done {
			return err
		}

		account, err := db.CreateAccount(ctx, owner.ID, "Kingfisher Collection")
		if err != nil {
			return err
		}
		t := tenant.Tenant{AccountID: account.ID, UserID: owner.ID, MembershipID: account.MembershipID, Role: domain.RoleOwner}

		villa, err := createProperty(ctx, db, t, domain.NewProperty{
			Slug: "kingfisher", Name: "Kingfisher Villa", BookingType: domain.BookingEntire,
			Location: "Unawatuna, Galle", Currency: "USD", Setup: villaSetup(),
		})
		if err != nil {
			return err
		}
		house, err := createProperty(ctx, db, t, domain.NewProperty{
			Slug: "coralbay", Name: "Coral Bay House", BookingType: domain.BookingRooms,
			Location: "Mirissa, Matara", Currency: "USD", Setup: guesthouseSetup(),
		})
		if err != nil {
			return err
		}

		for _, m := range team {
			u, _, err := db.UpsertUserByPhone(ctx, m.phone)
			if err != nil {
				return err
			}
			name := m.name
			if _, err := db.UpdateUser(ctx, u.ID, domain.UserPatch{Name: &name}); err != nil {
				return err
			}
			var scope []uuid.UUID
			if m.villaOnly {
				scope = []uuid.UUID{villa.ID}
			}
			if _, err := db.AddMember(ctx, t, u.ID, m.role, scope); err != nil {
				return err
			}
			summary = append(summary, fmt.Sprintf("  %-9s %-17s %s", m.role, m.name, m.phone))
		}

		token, hash, err := inviteToken()
		if err != nil {
			return err
		}
		if err := db.CreateInvite(ctx, t, store.Invite{
			Phone: pendingInvite.phone, Role: pendingInvite.role, PropertyIDs: []uuid.UUID{villa.ID},
			TokenHash: hash, ExpiresAt: time.Now().Add(14 * 24 * time.Hour),
		}); err != nil {
			return err
		}

		summary = append([]string{
			fmt.Sprintf("Seeded account %s (Kingfisher Collection)", account.ID),
			fmt.Sprintf("  owner     %-17s %s", ownerName, ownerPhone),
		}, summary...)
		summary = append(summary,
			fmt.Sprintf("  invited   caretaker         %s (staykey://join?token=%s)", pendingInvite.phone, token),
			fmt.Sprintf("Properties: %s (%s), %s (%s)", villa.Name, villa.Slug, house.Name, house.Slug),
		)
		return nil
	})
	if err != nil {
		return err
	}
	for _, line := range summary {
		fmt.Println(line)
	}
	return nil
}

// clearPrevious finds an earlier sample account by its villa's address. Without reset it reports
// that seeding is already done; with reset it deletes that account, but only when ownerID owns it.
func clearPrevious(ctx context.Context, db *store.Postgres, ownerID uuid.UUID, reset bool) (done bool, err error) {
	villa, err := db.GetPropertyBySlug(ctx, "kingfisher")
	if errors.Is(err, domain.ErrNotFound) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	m, err := db.Membership(ctx, ownerID, villa.AccountID)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && m.Role != domain.RoleOwner) {
		return false, errors.New("the kingfisher address belongs to another account; free it or seed a fresh database")
	}
	if err != nil {
		return false, err
	}
	if !reset {
		fmt.Printf("Already seeded: account %s has Kingfisher Villa. Run with -reset to start over.\n", villa.AccountID)
		return true, nil
	}
	if err := db.DeleteAccount(ctx, tenant.FromMembership(m)); err != nil {
		return false, err
	}
	fmt.Printf("Deleted the previous sample account %s.\n", villa.AccountID)
	return false, nil
}

// createProperty checks the sample against the same rules as the API before storing it.
func createProperty(ctx context.Context, db *store.Postgres, t tenant.Tenant, p domain.NewProperty) (domain.PropertyDetail, error) {
	p.Normalize()
	if err := p.Validate(); err != nil {
		return domain.PropertyDetail{}, fmt.Errorf("sample %s: %w", p.Name, err)
	}
	created, err := db.CreateProperty(ctx, t, p)
	if err != nil {
		return domain.PropertyDetail{}, fmt.Errorf("create %s: %w", p.Name, err)
	}
	return created, nil
}

// villaSetup and guesthouseSetup mirror sampleVilla, sampleGuesthouse and withSampleSettings in
// apps/mobile/src/data/seed.ts.
func villaSetup() domain.PropertyPatch {
	units := []domain.UnitInput{
		{Ref: "villa", Name: "Kingfisher Villa", Sleeps: 6, Beds: "3 bedrooms", Rate: 18000, WeekendRate: 22000},
	}
	p := sampleSettings(units, "Three-bedroom villa with a private pool, ten minutes from the beach.")
	p.ExtraGuest = &domain.ExtraGuest{Above: 4, Amount: 2500}
	p.IcalFeeds = &[]domain.IcalFeedInput{
		{Ref: "airbnb", Channel: "airbnb", URL: "https://www.airbnb.com/calendar/ical/41829.ics"},
		{Ref: "booking", Channel: "booking", URL: "https://admin.booking.com/hotel/hoteladmin/ical.html?t=7f3a"},
	}
	return p
}

func guesthouseSetup() domain.PropertyPatch {
	units := []domain.UnitInput{
		{Ref: "ocean", Name: "Ocean Room", Sleeps: 2, Beds: "1 king bed", Rate: 9000, WeekendRate: 10000},
		{Ref: "garden", Name: "Garden Room", Sleeps: 2, Beds: "2 single beds", Rate: 7500},
		{Ref: "family", Name: "Family Suite", Sleeps: 4, Beds: "1 king and 2 single beds", Rate: 13000},
		{Ref: "whole", Name: "Whole house", Sleeps: 8, Beds: "Books all three rooms together", Rate: 28000,
			LinkedUnitRef: []string{"ocean", "garden", "family"}},
	}
	p := sampleSettings(units, "A small guesthouse with garden rooms, a short walk from the beach.")
	policy := "flexible"
	p.Policy = &policy
	return p
}

func sampleSettings(units []domain.UnitInput, description string) domain.PropertyPatch {
	season := func(ref, name, start, end string, factor float64, minNights int) domain.SeasonInput {
		prices := map[string]int64{}
		for _, u := range units {
			prices[u.Ref] = int64(math.Round(float64(u.Rate)*factor/1000)) * 1000
		}
		return domain.SeasonInput{Ref: ref, Name: name, Start: start, End: end, MinNights: minNights, Prices: prices}
	}
	nextYear := time.Now().Year() + 1
	amenities := []string{"Pool", "WiFi", "Air conditioning", "Kitchen"}
	return domain.PropertyPatch{
		Description: &description,
		Amenities:   &amenities,
		Units:       &units,
		Seasons: &[]domain.SeasonInput{
			season("peak", "Peak season", "12-15", "01-15", 1.75, 3),
			season("high", "High season", "01-16", "04-30", 1.45, 0),
			season("monsoon", "Monsoon", "05-01", "08-31", 0.8, 0),
		},
		LengthDiscounts: &[]domain.LengthDiscount{{Nights: 7, Percent: 10}, {Nights: 28, Percent: 25}},
		Charges: &[]domain.ChargeInput{
			{Ref: "service", Charge: domain.Charge{Name: "Service charge", Kind: "percent", Amount: 10, Per: "stay", Enabled: true, Note: "on the room rate"}},
			{Ref: "vat", Charge: domain.Charge{Name: "VAT", Kind: "percent", Amount: 18, Per: "stay", Note: "on the room rate, once you register"}},
			{Ref: "cleaning", Charge: domain.Charge{Name: "Cleaning fee", Kind: "fixed", Amount: 3000, Per: "stay", Enabled: true}},
		},
		Extras: &[]domain.ExtraInput{
			{Ref: "transfer", Extra: domain.Extra{Name: "Airport transfer", Price: 4000, Per: "trip", Enabled: true}},
			{Ref: "breakfast", Extra: domain.Extra{Name: "Breakfast", Price: 800, Per: "guestNight", Enabled: true}},
			{Ref: "tour", Extra: domain.Extra{Name: "Galle Fort tuk-tuk tour", Price: 2500, Per: "guest", Enabled: true}},
			{Ref: "late", Extra: domain.Extra{Name: "Late check-out", Price: 3000, Per: "stay", OnRequest: true}},
		},
		Promos: &[]domain.PromoInput{
			{Ref: "return", Promo: domain.Promo{Code: "RETURN10", Kind: "percent", Amount: 10, Limit: 20, Note: "For returning guests"}},
			{Ref: "monsoon", Promo: domain.Promo{Code: "MONSOON50", Kind: "fixed", Amount: 5000, MinNights: 3,
				From: fmt.Sprintf("%d-05-01", nextYear), To: fmt.Sprintf("%d-08-31", nextYear)}},
		},
		Payments: &domain.PaymentSettings{
			Bank: domain.BankDetails{Enabled: true, BankName: "Commercial Bank", AccountName: "N. Perera",
				AccountNumber: "8001234417", PayWithinHours: 24, CancelIfUnpaid: true},
			AtProperty: true,
			Cards:      "off",
		},
	}
}

func inviteToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}
