package jobs

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/store/storetest"
	"staykey.direct/api/internal/tenant"
)

func TestSweepUploadsRemovesOldUnusedFiles(t *testing.T) {
	ctx := context.Background()
	now := time.Date(2026, 9, 1, 12, 0, 0, 0, time.UTC)
	st := storetest.New(t)
	fs, err := files.NewLocal(t.TempDir(), []byte("test"))
	if err != nil {
		t.Fatal(err)
	}
	u, _, err := st.UpsertUserByPhone(ctx, "+94771234567")
	if err != nil {
		t.Fatal(err)
	}
	acc, err := st.CreateAccount(ctx, u.ID, "Sweeps")
	if err != nil {
		t.Fatal(err)
	}
	tn := tenant.Tenant{AccountID: acc.ID, UserID: u.ID, Role: domain.RoleOwner}
	p, err := st.CreateProperty(ctx, tn, domain.NewProperty{Slug: "sweeps", Name: "Sweeps", BookingType: domain.BookingEntire, Currency: "USD", BaseRateMinor: 10000})
	if err != nil {
		t.Fatal(err)
	}
	unit := p.Units[0].ID
	b, err := st.CreateBooking(ctx, tn, domain.NewBooking{
		PropertyID: p.ID, Status: domain.StatusConfirmed, Currency: "USD", RefPrefix: "SW", LedgerUnits: []uuid.UUID{unit},
		Stay: domain.StayInput{UnitID: unit, CheckIn: now, CheckOut: now.AddDate(0, 0, 1), Adults: 1, Guest: domain.Guest{Name: "Guest"}, Source: "phone"},
	}, now)
	if err != nil {
		t.Fatal(err)
	}

	slip := upload(t, fs, acc.ID, files.KindSlip, now.Add(-48*time.Hour))
	if _, err := st.AddSlip(ctx, tn, b.ID, slip, 5000, now); err != nil {
		t.Fatal(err)
	}
	upload(t, fs, acc.ID, files.KindPhoto, now.Add(-48*time.Hour))
	fresh := upload(t, fs, acc.ID, files.KindPhoto, now.Add(-time.Hour))

	if err := SweepUploads(ctx, st, fs, now); err != nil {
		t.Fatal(err)
	}

	left, err := fs.WrittenBefore(ctx, now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{slip, fresh}
	slices.Sort(left)
	slices.Sort(want)
	if !slices.Equal(left, want) {
		t.Errorf("files left = %v, want the slip and the fresh photo %v", left, want)
	}
}

// upload writes a file as if a client had uploaded it at the given time.
func upload(t *testing.T, fs *files.Local, account uuid.UUID, kind files.Kind, at time.Time) string {
	t.Helper()
	key, err := files.NewKey(account, kind, "image/jpeg")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(fs.Dir, filepath.FromSlash(key))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("jpeg"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, at, at); err != nil {
		t.Fatal(err)
	}
	return key
}
