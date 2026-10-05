package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/domain/pricing"
	"staykey.direct/api/internal/files"
	"staykey.direct/api/internal/store"
	"staykey.direct/api/internal/tenant"
)

// plan is one sample stay, mirroring PLANS in apps/mobile/src/data/seed.ts. Days are relative
// to today at the property.
type plan struct {
	unit                 int
	from, nights         int
	source               domain.Source
	status               domain.BookingStatus
	name, phone, email   string
	country              string
	adults, children     int
	extras               []string // extra names
	paid                 string   // full, deposit or empty
	method               string
	slip                 bool
	guestNote, ownerNote string
	expiresInHours       int
}

var plans = []plan{
	{unit: 0, from: -14, nights: 4, source: "page", status: domain.StatusCheckedOut, name: "Hannah Clarke", country: "GB", paid: "full", method: "card"},
	{unit: 0, from: -3, nights: 3, source: "widget", status: domain.StatusCheckedIn, name: "Sofia Rossi", phone: "+39 347 555 0142",
		email: "sofia.rossi@example.com", country: "IT", paid: "full", method: "card"},
	{unit: 0, from: 0, nights: 4, source: "page", status: domain.StatusConfirmed, name: "Emma Larsen", phone: "+44 7700 900123",
		email: "emma.larsen@example.com", country: "GB", extras: []string{"Airport transfer"}, paid: "full", method: "card",
		guestNote: "We land at 11:30. Could we check in a little early?", ownerNote: "Leave pool towels out"},
	{unit: 0, from: 6, nights: 3, source: "booking", status: domain.StatusConfirmed},
	{unit: 0, from: 12, nights: 3, source: "page", status: domain.StatusRequested, name: "Lukas Weber", phone: "+49 151 2345 6789",
		email: "lukas.weber@example.com", country: "DE", expiresInHours: 18},
	{unit: 0, from: 18, nights: 5, source: "airbnb", status: domain.StatusConfirmed},
	{unit: 0, from: 24, nights: 3, source: "whatsapp", status: domain.StatusConfirmed, name: "Daniel Fernando", phone: "+94 77 123 4567",
		country: "LK", adults: 4, paid: "deposit", method: "bank", slip: true},
	{unit: 0, from: 29, nights: 3, source: "page", status: domain.StatusConfirmed, name: "Mia Schneider", phone: "+49 160 555 0199",
		email: "mia.schneider@example.com", country: "DE", paid: "full", method: "card"},
	{unit: 0, from: 40, nights: 3, source: "page", status: domain.StatusCancelled, name: "Tom Becker", email: "tom.becker@example.com", country: "DE"},
	{unit: 1, from: -6, nights: 4, source: "page", status: domain.StatusCheckedOut, name: "Chen Wei", country: "SG", paid: "full", method: "card"},
	{unit: 1, from: 0, nights: 3, source: "whatsapp", status: domain.StatusConfirmed, name: "Arjun Mehta", phone: "+91 98200 55501",
		country: "IN", paid: "deposit", method: "bank", slip: true},
	{unit: 1, from: 8, nights: 4, source: "booking", status: domain.StatusConfirmed},
	{unit: 2, from: 2, nights: 3, source: "widget", status: domain.StatusAwaitingPayment, name: "Olivia Brown", email: "olivia.brown@example.com", country: "AU"},
	{unit: 2, from: 6, nights: 2, source: "page", status: domain.StatusRequested, name: "Priya Nair", phone: "+91 99000 12345",
		country: "IN", adults: 2, children: 1, expiresInHours: 20},
}

// history is a few months of past stays on the first unit, so Insights has something to show.
var history = []struct {
	source domain.Source
	name   string
}{
	{"page", "Anna Svensson"}, {"booking", ""}, {"airbnb", ""}, {"whatsapp", "Kavindu Silva"}, {"widget", "Lena Hoffmann"},
	{"page", "James Wilson"}, {"booking", ""}, {"page", "Ayesha Khan"}, {"airbnb", ""}, {"page", "Marco Bianchi"},
}

// altNames give the second property its own guests, as sampleData does with alt in seed.ts.
var altNames = map[string]string{
	"Emma Larsen": "Chloe Martin", "Sofia Rossi": "Elena Garcia", "Hannah Clarke": "Olga Petrova", "Lukas Weber": "Tomas Novak",
	"Daniel Fernando": "Nimal Wijesinghe", "Mia Schneider": "Julia Nowak", "Tom Becker": "Ben Carter", "Anna Svensson": "Freya Olsen",
	"Kavindu Silva": "Ruvini Fonseka", "Lena Hoffmann": "Clara Weiss", "James Wilson": "Oliver Grant", "Ayesha Khan": "Sana Malik",
	"Marco Bianchi": "Luca Romano",
}

// seedStays adds the sample stays and the pool resurfacing block to a property. It returns how
// many stays it added.
func seedStays(ctx context.Context, db *store.Postgres, t tenant.Tenant, p domain.PropertyDetail, alt bool) (int, error) {
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		return 0, err
	}
	now := time.Now()
	y, m, d := now.In(loc).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	var units []domain.Unit
	for _, u := range p.Units {
		if len(u.LinkedUnitIDs) == 0 {
			units = append(units, u)
		}
	}
	rename := func(name string) string {
		if n, ok := altNames[name]; ok && alt {
			return n
		}
		return name
	}

	added := 0
	for _, pl := range plans {
		if pl.unit >= len(units) {
			continue
		}
		pl.name = rename(pl.name)
		if alt && pl.email != "" {
			pl.email = strings.ToLower(strings.ReplaceAll(pl.name, " ", ".")) + "@example.com"
		}
		if alt {
			// Guests are matched by phone, so the second property's guests go without one.
			pl.phone = ""
		}
		b, err := addStay(ctx, db, t, p, units[pl.unit], today.AddDate(0, 0, pl.from), pl, now)
		if err != nil {
			return added, err
		}
		added++
		if pl.slip {
			key, err := files.NewKey(t.AccountID, files.KindSlip, "image/jpeg")
			if err != nil {
				return added, err
			}
			if _, err := db.AddSlip(ctx, t, b.ID, key, b.Total-b.Paid(), now.Add(-3*time.Hour)); err != nil {
				return added, fmt.Errorf("add slip: %w", err)
			}
		}
		if pl.status == domain.StatusCancelled {
			if _, err := db.CancelBooking(ctx, t, b.ID, b.Version, "Guest changed plans", nil, now); err != nil {
				return added, fmt.Errorf("cancel %s: %w", b.Ref, err)
			}
		}
	}
	for k, h := range history {
		pl := plan{nights: 3 + k%3, source: h.source, status: domain.StatusCheckedOut, name: rename(h.name), paid: "full", method: "card"}
		if h.source == "whatsapp" {
			pl.method = "bank"
		}
		if _, err := addStay(ctx, db, t, p, units[0], today.AddDate(0, 0, -22-k*16), pl, now); err != nil {
			return added, err
		}
		added++
	}

	block := domain.Block{PropertyID: p.ID, UnitIDs: []uuid.UUID{units[0].ID}, From: today.AddDate(0, 0, 10),
		To: today.AddDate(0, 0, 11), Reason: "maintenance", Note: "Pool resurfacing"}
	if _, err := db.CreateBlock(ctx, t, block, domain.LedgerUnits(p.Units, units[0].ID)); err != nil {
		return added, fmt.Errorf("add block: %w", err)
	}
	return added, nil
}

func addStay(ctx context.Context, db *store.Postgres, t tenant.Tenant, p domain.PropertyDetail, unit domain.Unit, checkIn time.Time, pl plan, now time.Time) (domain.Booking, error) {
	adults := pl.adults
	if adults == 0 {
		adults = 2
	}
	adults = min(adults, unit.Sleeps)
	stay := domain.StayInput{
		UnitID: unit.ID, CheckIn: checkIn, CheckOut: checkIn.AddDate(0, 0, pl.nights), Adults: adults, Children: pl.children,
		Guest:  domain.Guest{Name: pl.name, Phone: pl.phone, Email: pl.email, Country: pl.country},
		Source: pl.source, OwnerNote: pl.ownerNote,
	}
	for _, name := range pl.extras {
		for _, e := range p.Extras {
			if e.Name == name {
				stay.Extras = append(stay.Extras, e.ID)
			}
		}
	}
	if pl.source.OTA() {
		stay.Guest = domain.Guest{Name: "Airbnb stay"}
		if pl.source == "booking" {
			stay.Guest.Name = "Booking.com stay"
		}
	}
	stay.Normalize()

	var price domain.Pricing
	if !pl.source.OTA() {
		q := pricing.Price(p, pricing.Input{UnitID: unit.ID, From: stay.CheckIn, To: stay.CheckOut, Adults: adults, Children: pl.children, Extras: stay.Extras}, nil)
		price = domain.Pricing{Lines: q.Lines, Extras: q.Extras, Total: q.Total}
	}
	status := pl.status
	if status == domain.StatusCancelled {
		status = domain.StatusConfirmed
	}
	in := domain.NewBooking{
		PropertyID: p.ID, Stay: stay, Status: status, Currency: p.Currency, RefPrefix: domain.RefPrefix(p.Name),
		Price: price, LedgerUnits: domain.LedgerUnits(p.Units, unit.ID), GuestNote: pl.guestNote,
	}
	if pl.expiresInHours > 0 {
		at := now.Add(time.Duration(pl.expiresInHours) * time.Hour)
		in.RequestExpiresAt = &at
	}
	switch pl.paid {
	case "full":
		in.FirstPayment = &domain.Payment{Method: pl.method, Amount: price.Total}
	case "deposit":
		// Deposits round to whole currency units, as guests pay them.
		in.FirstPayment = &domain.Payment{Method: pl.method, Amount: pricing.Deposit(p, price.Total) / 100 * 100}
	}
	if in.FirstPayment != nil && in.FirstPayment.Amount == 0 {
		in.FirstPayment = nil
	}
	b, err := db.CreateBooking(ctx, t, in, now)
	if err != nil {
		return domain.Booking{}, fmt.Errorf("add %s's stay: %w", stay.Guest.Name, err)
	}
	return b, nil
}
