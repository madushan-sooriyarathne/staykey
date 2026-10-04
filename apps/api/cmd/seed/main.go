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
// Phase 1 covers what the schema holds so far: the account, people, the two properties with
// their base rate, and memberships. Units, seasons, settings and sample bookings join as their
// tables arrive in phases 2 and 3.
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

		villa, err := db.CreateProperty(ctx, t, domain.NewProperty{
			Slug: "kingfisher", Name: "Kingfisher Villa", BookingType: domain.BookingEntire,
			Location: "Unawatuna, Galle", Currency: "USD", BaseRateMinor: 18000,
		})
		if err != nil {
			return fmt.Errorf("create Kingfisher Villa: %w", err)
		}
		// Rooms from 75 USD a night; the base rate is the lowest room.
		house, err := db.CreateProperty(ctx, t, domain.NewProperty{
			Slug: "coralbay", Name: "Coral Bay House", BookingType: domain.BookingRooms,
			Location: "Mirissa, Matara", Currency: "USD", BaseRateMinor: 7500,
		})
		if err != nil {
			return fmt.Errorf("create Coral Bay House: %w", err)
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

func inviteToken() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	token := base64.RawURLEncoding.EncodeToString(b)
	h := sha256.Sum256([]byte(token))
	return token, h[:], nil
}
