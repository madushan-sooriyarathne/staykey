package server

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
	"staykey.direct/api/internal/tenant"
)

// guesthouse is a full onboarding publish: rooms with a whole-house unit, seasons priced per
// room by the ids the units were sent with, and every setting section.
func guesthouse() map[string]any {
	return map[string]any{
		"name": "Coral Bay House", "slug": "coralbay", "bookingType": "rooms",
		"location": "Mirissa, Matara", "currency": "USD",
		"setup": map[string]any{
			"checkIn": "14:00", "checkOut": "11:00", "policy": "flexible", "depositPercent": 30,
			"houseRules": []string{"No smoking indoors", "Pets are welcome"},
			"units": []map[string]any{
				{"id": "ocean", "name": "Ocean Room", "sleeps": 2, "beds": "1 king bed", "rate": 9000, "weekendRate": 10000},
				{"id": "garden", "name": "Garden Room", "sleeps": 2, "rate": 7500},
				{"id": "whole", "name": "Whole house", "sleeps": 4, "rate": 15000, "linkedUnitIds": []string{"ocean", "garden"}},
			},
			"seasons": []map[string]any{
				{"name": "Peak season", "start": "12-15", "end": "01-15", "minNights": 3, "prices": map[string]int{"ocean": 15000, "whole": 26000}},
			},
			"lengthDiscounts": []map[string]any{{"nights": 7, "percent": 10}},
			"extras":          []map[string]any{{"name": "Airport transfer", "price": 4000, "per": "trip", "onRequest": false, "enabled": true}},
			"promos":          []map[string]any{{"code": "monsoon10", "kind": "percent", "amount": 10, "from": "2026-05-01", "to": "2026-08-31"}},
			"payments": map[string]any{
				"bank": map[string]any{"enabled": true, "bankName": "Sampath Bank", "accountName": "N Perera",
					"accountNumber": "0012 3456 7890", "payWithinHours": 24, "cancelIfUnpaid": true},
				"atProperty": true, "cards": "off",
			},
			"booking":   map[string]any{"mode": "request", "replyHours": 24, "holdMinutes": 15, "displayCurrencies": []string{"USD", "EUR"}},
			"icalFeeds": []map[string]any{{"channel": "airbnb", "url": ""}, {"channel": "booking", "url": "https://admin.booking.com/ical/1.ics"}},
		},
	}
}

func unitByName(t *testing.T, p oapi.Property, name string) oapi.Unit {
	t.Helper()
	for _, u := range p.Units {
		if u.Name == name {
			return u
		}
	}
	t.Fatalf("no unit %q in %+v", name, p.Units)
	return oapi.Unit{}
}

func TestPublishFullSetup(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94774000001", "Coral Bay")

	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: guesthouse(), as: s, account: account})
	e.expect(rec, http.StatusCreated)
	p := decode[oapi.Property](t, rec)

	if p.BaseRate != 7500 {
		t.Errorf("base rate = %d, want the lowest unit rate 7500", p.BaseRate)
	}
	ocean, garden, whole := unitByName(t, p, "Ocean Room"), unitByName(t, p, "Garden Room"), unitByName(t, p, "Whole house")
	if got := whole.LinkedUnitIds; len(got) != 2 || !contains(got, ocean.Id) || !contains(got, garden.Id) {
		t.Errorf("whole house links %v, want ocean and garden", got)
	}
	if ocean.WeekendRate == nil || *ocean.WeekendRate != 10000 || garden.WeekendRate != nil {
		t.Errorf("weekend rates %v / %v", ocean.WeekendRate, garden.WeekendRate)
	}
	if len(p.Seasons) != 1 || p.Seasons[0].Prices[ocean.Id.String()] != 15000 || p.Seasons[0].Prices[whole.Id.String()] != 26000 {
		t.Errorf("season prices %+v", p.Seasons)
	}
	if len(p.Charges) != 2 || p.Charges[0].Name != "Service charge" || p.Charges[0].Enabled {
		t.Errorf("default charges %+v", p.Charges)
	}
	if len(p.Promos) != 1 || p.Promos[0].Code != "MONSOON10" || p.Promos[0].From == nil {
		t.Errorf("promos %+v", p.Promos)
	}
	if p.Payments.Bank.AccountNumber != "001234567890" || p.Payments.Cards != oapi.PaymentSettingsCardsOff {
		t.Errorf("payments %+v", p.Payments)
	}
	if p.Booking.Mode != oapi.Request || len(p.Booking.DisplayCurrencies) != 2 || p.Rules.MinNights != 1 {
		t.Errorf("booking %+v, rules %+v", p.Booking, p.Rules)
	}
	if len(p.IcalFeeds) != 2 || p.IcalFeeds[1].Status != oapi.IcalFeedStatusPending || p.IcalExportToken == "" {
		t.Errorf("feeds %+v, token %q", p.IcalFeeds, p.IcalExportToken)
	}
	if p.HouseRules[1] != "Pets are welcome" || p.Policy != oapi.Flexible {
		t.Errorf("policies: %v %s", p.HouseRules, p.Policy)
	}

	// The account number is encrypted at rest.
	var raw []byte
	if err := e.store.Pool().QueryRow(context.Background(),
		"SELECT account_number_enc FROM payment_settings WHERE property_id = $1", p.Id).Scan(&raw); err != nil {
		t.Fatal(err)
	}
	if len(raw) == 0 || bytes.Contains(raw, []byte("001234567890")) {
		t.Errorf("account number stored as %q", raw)
	}

	// GET and the list return the same setup.
	rec = e.do(call{method: http.MethodGet, path: "/v1/properties/" + p.Id.String(), as: s, account: account})
	e.expect(rec, http.StatusOK)
	if got := decode[oapi.Property](t, rec); len(got.Units) != 3 || got.Payments.Bank.AccountNumber != "001234567890" {
		t.Errorf("get: %d units, account %q", len(got.Units), got.Payments.Bank.AccountNumber)
	}
}

func TestPatchReplacesOnlyWhatItSends(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94774000002", "Coral Bay")
	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: guesthouse(), as: s, account: account})
	e.expect(rec, http.StatusCreated)
	p := decode[oapi.Property](t, rec)
	path := "/v1/properties/" + p.Id.String()
	patch := func(body map[string]any) oapi.Property {
		t.Helper()
		rec := e.do(call{method: http.MethodPatch, path: path, body: body, as: s, account: account})
		e.expect(rec, http.StatusOK)
		return decode[oapi.Property](t, rec)
	}

	// A scalar alone leaves every other section as it was.
	got := patch(map[string]any{"name": "Coral Bay Guesthouse", "description": "  Rooms by the beach. "})
	if got.Name != "Coral Bay Guesthouse" || got.Description != "Rooms by the beach." || len(got.Units) != 3 || len(got.Promos) != 1 {
		t.Fatalf("after name patch: %+v", got)
	}

	// Dropping a room archives it, takes it out of the whole house and its season prices.
	ocean, whole := unitByName(t, got, "Ocean Room"), unitByName(t, got, "Whole house")
	got = patch(map[string]any{"units": []map[string]any{
		{"id": ocean.Id.String(), "name": "Ocean Suite", "sleeps": 3, "rate": 11000},
		{"id": whole.Id.String(), "name": "Whole house", "sleeps": 3, "rate": 15000, "linkedUnitIds": []string{ocean.Id.String()}},
		{"id": "new-attic", "name": "Attic Room", "sleeps": 2, "rate": 6000},
	}})
	if len(got.Units) != 3 || unitByName(t, got, "Ocean Suite").Id != ocean.Id {
		t.Fatalf("units after patch: %+v", got.Units)
	}
	if links := unitByName(t, got, "Whole house").LinkedUnitIds; len(links) != 1 || links[0] != ocean.Id {
		t.Errorf("links after patch: %v", links)
	}
	if got.BaseRate != 6000 {
		t.Errorf("base rate %d, want 6000", got.BaseRate)
	}
	var archived int
	_ = e.store.Pool().QueryRow(context.Background(),
		"SELECT count(*) FROM units WHERE property_id = $1 AND archived_at IS NOT NULL", p.Id).Scan(&archived)
	if archived != 1 {
		t.Errorf("%d archived units, want 1 (the garden room)", archived)
	}

	// Charges keep their ids when edited; items without an id are added; left-out ones go.
	service := got.Charges[0]
	got = patch(map[string]any{"charges": []map[string]any{
		{"id": service.Id.String(), "name": "Service charge", "kind": "percent", "amount": 10, "per": "stay", "enabled": true},
		{"name": "Cleaning fee", "kind": "fixed", "amount": 3000, "per": "stay", "enabled": true},
	}})
	if len(got.Charges) != 2 || got.Charges[0].Id != service.Id || !got.Charges[0].Enabled || got.Charges[1].Name != "Cleaning fee" {
		t.Errorf("charges: %+v", got.Charges)
	}

	// A promo's used count belongs to bookings, so edits keep it.
	promo := got.Promos[0]
	if _, err := e.store.Pool().Exec(context.Background(), "UPDATE promos SET used_count = 3 WHERE id = $1", promo.Id); err != nil {
		t.Fatal(err)
	}
	got = patch(map[string]any{"promos": []map[string]any{
		{"id": promo.Id.String(), "code": "MONSOON15", "kind": "percent", "amount": 15},
	}})
	if got.Promos[0].Id != promo.Id || got.Promos[0].Used != 3 || got.Promos[0].Code != "MONSOON15" {
		t.Errorf("promo: %+v", got.Promos[0])
	}

	// Rules and booking settings, then turning extra guests on and off again.
	got = patch(map[string]any{
		"rules":      map[string]any{"minNights": 2, "maxNights": 14, "sameDayCutoff": nil, "windowMonths": 6, "closedArrival": []int{5, 0}},
		"extraGuest": map[string]any{"above": 2, "amount": 2500},
	})
	if got.Rules.SameDayCutoff != nil || got.Rules.ClosedArrival[0] != 0 || got.ExtraGuest.Amount != 2500 {
		t.Errorf("rules %+v, extra guest %+v", got.Rules, got.ExtraGuest)
	}
	got = patch(map[string]any{"extraGuest": map[string]any{"above": 2, "amount": 0}})
	if got.ExtraGuest.Amount != 0 || got.ExtraGuest.Above != 0 {
		t.Errorf("extra guest off: %+v", got.ExtraGuest)
	}

	// A changed feed link starts over as pending.
	feed := got.IcalFeeds[1]
	if _, err := e.store.Pool().Exec(context.Background(), "UPDATE ical_feeds SET status = 'ok' WHERE id = $1", feed.Id); err != nil {
		t.Fatal(err)
	}
	got = patch(map[string]any{"icalFeeds": []map[string]any{
		{"id": feed.Id.String(), "channel": "booking", "url": "https://admin.booking.com/ical/2.ics"},
	}})
	if len(got.IcalFeeds) != 1 || got.IcalFeeds[0].Status != oapi.IcalFeedStatusPending {
		t.Errorf("feeds: %+v", got.IcalFeeds)
	}
}

func TestPatchValidation(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94774000003", "Villa")
	p := e.createProperty(s, account, "Kingfisher Villa")
	path := "/v1/properties/" + p.Id.String()

	cases := map[string]struct {
		body  map[string]any
		field string
	}{
		"unknown season unit": {map[string]any{"seasons": []map[string]any{
			{"name": "Peak", "start": "12-15", "end": "01-15", "prices": map[string]int{"nowhere": 100}},
		}}, "seasons[0].prices"},
		"bank without details": {map[string]any{"payments": map[string]any{
			"bank":       map[string]any{"enabled": true, "bankName": "", "accountName": "", "accountNumber": "", "payWithinHours": 24, "cancelIfUnpaid": true},
			"atProperty": true, "cards": "off",
		}}, "payments.bank.bankName"},
		"cards approved by the app": {map[string]any{"payments": map[string]any{
			"bank":       map[string]any{"enabled": false, "bankName": "", "accountName": "", "accountNumber": "", "payWithinHours": 24, "cancelIfUnpaid": true},
			"atProperty": true, "cards": "on",
		}}, "payments.cards"},
		"same code twice": {map[string]any{"promos": []map[string]any{
			{"code": "SUMMER", "kind": "percent", "amount": 10}, {"code": "summer", "kind": "fixed", "amount": 500},
		}}, "promos[1].code"},
		"no units": {map[string]any{"units": []map[string]any{}}, "units"},
		"bad time": {map[string]any{"checkIn": "25:00"}, ""},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			rec := e.do(call{method: http.MethodPatch, path: path, body: c.body, as: s, account: account})
			e.expect(rec, http.StatusBadRequest)
			body := decode[oapi.Error](t, rec)
			if c.field != "" && (body.Field == nil || *body.Field != c.field) {
				t.Errorf("field = %v (%s), want %s", body.Field, body.Message, c.field)
			}
		})
	}

	// Nothing changed.
	rec := e.do(call{method: http.MethodGet, path: path, as: s, account: account})
	if got := decode[oapi.Property](t, rec); len(got.Seasons) != 0 || len(got.Promos) != 0 || len(got.Units) != 1 {
		t.Errorf("a failed patch changed the property: %+v", got)
	}
}

func TestSettingsPermissions(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	owner, account := e.owner("+94774000004", "Two villas")
	rec := e.do(call{method: http.MethodPost, path: "/v1/properties", body: guesthouse(), as: owner, account: account})
	e.expect(rec, http.StatusCreated)
	house := decode[oapi.Property](t, rec)
	villa := e.createProperty(owner, account, "Kingfisher Villa")

	ownerTenant := tenant.Tenant{AccountID: uuid.MustParse(account), UserID: owner.userID, Role: domain.RoleOwner}
	manager, _ := e.signIn("+94774000005")
	caretaker, _ := e.signIn("+94774000006")
	if _, err := e.store.AddMember(context.Background(), ownerTenant, manager.userID, domain.RoleManager, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := e.store.AddMember(context.Background(), ownerTenant, caretaker.userID, domain.RoleCaretaker, []uuid.UUID{house.Id}); err != nil {
		t.Fatal(err)
	}
	housePath := "/v1/properties/" + house.Id.String()
	ratePatch := map[string]any{"lengthDiscounts": []map[string]any{{"nights": 14, "percent": 15}}}

	// Managers run the settings, prices included.
	e.expect(e.do(call{method: http.MethodPatch, path: housePath, body: ratePatch, as: manager, account: account}), http.StatusOK)

	// Caretakers can't change anything, and don't see bank details or promo codes.
	e.expect(e.do(call{method: http.MethodPatch, path: housePath, body: map[string]any{"checkIn": "15:00"}, as: caretaker, account: account}), http.StatusForbidden)
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/uploads", body: map[string]any{"kind": "photo", "contentType": "image/jpeg", "size": 10}, as: caretaker, account: account}), http.StatusForbidden)
	rec = e.do(call{method: http.MethodGet, path: housePath, as: caretaker, account: account})
	e.expect(rec, http.StatusOK)
	seen := decode[oapi.Property](t, rec)
	if seen.Payments.Bank.AccountNumber != "" || seen.Payments.Bank.AccountName != "" || len(seen.Promos) != 0 {
		t.Errorf("caretaker sees bank %+v and promos %+v", seen.Payments.Bank, seen.Promos)
	}
	if len(seen.Units) != 3 {
		t.Errorf("caretaker sees %d units, want 3", len(seen.Units))
	}

	// And only their properties.
	e.expect(e.do(call{method: http.MethodGet, path: "/v1/properties/" + villa.Id.String(), as: caretaker, account: account}), http.StatusNotFound)
}

func TestPhotoUploads(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	s, account := e.owner("+94774000007", "Villa")
	p := e.createProperty(s, account, "Kingfisher Villa")
	path := "/v1/properties/" + p.Id.String()
	image := []byte("\xff\xd8\xff pretend jpeg")

	ticket := func(size int) oapi.UploadTicket {
		t.Helper()
		rec := e.do(call{method: http.MethodPost, path: "/v1/uploads", as: s, account: account,
			body: map[string]any{"kind": "photo", "contentType": "image/jpeg", "size": size}})
		e.expect(rec, http.StatusCreated)
		return decode[oapi.UploadTicket](t, rec)
	}
	upload := func(tk oapi.UploadTicket, data []byte) int {
		t.Helper()
		u, _ := url.Parse(tk.UploadUrl)
		req := httptest.NewRequest(http.MethodPut, u.RequestURI(), bytes.NewReader(data))
		for k, v := range tk.Headers {
			req.Header.Set(k, v)
		}
		rec := httptest.NewRecorder()
		e.h.ServeHTTP(rec, req)
		return rec.Code
	}

	first := ticket(len(image))
	if !strings.HasPrefix(first.Key, "accounts/"+account+"/photos/") {
		t.Fatalf("key %s", first.Key)
	}

	// A key that was never uploaded is refused.
	rec := e.do(call{method: http.MethodPatch, path: path, as: s, account: account,
		body: map[string]any{"photos": []map[string]any{{"key": first.Key}}}})
	e.expect(rec, http.StatusBadRequest)
	if body := decode[oapi.Error](t, rec); body.Code != "upload_missing" {
		t.Errorf("code %s, want upload_missing", body.Code)
	}

	if code := upload(first, image); code != http.StatusOK {
		t.Fatalf("upload: %d", code)
	}
	second := ticket(len(image))
	if code := upload(second, image); code != http.StatusOK {
		t.Fatalf("upload: %d", code)
	}
	rec = e.do(call{method: http.MethodPatch, path: path, as: s, account: account,
		body: map[string]any{"photos": []map[string]any{{"key": second.Key, "caption": "Pool"}, {"key": first.Key}}}})
	e.expect(rec, http.StatusOK)
	got := decode[oapi.Property](t, rec)
	if len(got.Photos) != 2 || got.Photos[0].Key != second.Key || got.Photos[0].Caption != "Pool" ||
		!strings.HasSuffix(got.Photos[0].Url, "/media/"+second.Key) {
		t.Fatalf("photos %+v", got.Photos)
	}

	// The stored file is served.
	req := httptest.NewRequest(http.MethodGet, "/media/"+second.Key, nil)
	res := httptest.NewRecorder()
	e.h.ServeHTTP(res, req)
	if res.Code != http.StatusOK || !bytes.Equal(res.Body.Bytes(), image) {
		t.Errorf("serve: %d", res.Code)
	}

	// Another account's file can't be attached, even after it is uploaded.
	other, otherAccount := e.owner("+94774000008", "Elsewhere")
	rec = e.do(call{method: http.MethodPost, path: "/v1/uploads", as: other, account: otherAccount,
		body: map[string]any{"kind": "photo", "contentType": "image/jpeg", "size": len(image)}})
	foreign := decode[oapi.UploadTicket](t, rec)
	upload(foreign, image)
	rec = e.do(call{method: http.MethodPatch, path: path, as: s, account: account,
		body: map[string]any{"photos": []map[string]any{{"key": foreign.Key}}}})
	e.expect(rec, http.StatusBadRequest)
	if body := decode[oapi.Error](t, rec); body.Code != "invalid_upload" {
		t.Errorf("code %s, want invalid_upload", body.Code)
	}

	// Too large, or not an image.
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/uploads", as: s, account: account,
		body: map[string]any{"kind": "photo", "contentType": "image/jpeg", "size": 16 << 20}}), http.StatusBadRequest)
	e.expect(e.do(call{method: http.MethodPost, path: "/v1/uploads", as: s, account: account,
		body: map[string]any{"kind": "photo", "contentType": "application/pdf", "size": 100}}), http.StatusBadRequest)
}

func contains(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}
