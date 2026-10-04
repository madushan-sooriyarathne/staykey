package domain

import (
	"errors"
	"testing"
)

func ptr[T any](v T) *T { return &v }

// validPatch touches every section with values the app can send.
func validPatch() PropertyPatch {
	cutoff := 18
	return PropertyPatch{
		Name:           ptr("Coral Bay House"),
		BookingType:    ptr(BookingRooms),
		Description:    ptr("Garden rooms near the beach."),
		Location:       ptr("Mirissa, Matara"),
		CheckIn:        ptr("14:00"),
		CheckOut:       ptr("11:00"),
		Amenities:      ptr([]string{"Pool", "WiFi"}),
		Policy:         ptr("flexible"),
		DepositPercent: ptr(30),
		BalanceDueDays: ptr(14),
		HouseRules:     ptr([]string{"No smoking indoors"}),
		Rules:          &StayRules{MinNights: 2, MaxNights: 30, SameDayCutoff: &cutoff, WindowMonths: 12, ClosedArrival: []int{0}},
		Booking:        &BookingSettings{Mode: "request", ReplyHours: 24, HoldMinutes: 15, DisplayCurrencies: []string{"USD", "EUR"}},
		Branding:       &Branding{Color: "#FF5A00"},
		Payments: &PaymentSettings{
			Bank:       BankDetails{Enabled: true, BankName: "Sampath Bank", AccountName: "N Perera", AccountNumber: "0012 3456 7890", PayWithinHours: 24, CancelIfUnpaid: true},
			AtProperty: true,
			Cards:      "pending",
		},
		ExtraGuest: &ExtraGuest{Above: 2, Amount: 2500},
		Units: ptr([]UnitInput{
			{Ref: "ocean", Name: "Ocean Room", Sleeps: 2, Rate: 9000, WeekendRate: 10000},
			{Ref: "garden", Name: "Garden Room", Sleeps: 2, Rate: 7500},
			{Ref: "whole", Name: "Whole house", Sleeps: 4, Rate: 15000, LinkedUnitRef: []string{"ocean", "garden"}},
		}),
		Seasons:         ptr([]SeasonInput{{Ref: "peak", Name: "Peak", Start: "12-15", End: "01-15", MinNights: 3, Prices: map[string]int64{"ocean": 15000}}}),
		LengthDiscounts: ptr([]LengthDiscount{{Nights: 28, Percent: 25}, {Nights: 7, Percent: 10}}),
		Charges:         ptr(DefaultCharges()),
		Extras:          ptr([]ExtraInput{{Ref: "x", Extra: Extra{Name: "Airport transfer", Price: 4000, Per: "trip", Enabled: true}}}),
		Promos:          ptr([]PromoInput{{Ref: "p", Promo: Promo{Code: "monsoon-10", Kind: "percent", Amount: 10, From: "2026-05-01", To: "2026-08-31"}}}),
		Photos:          ptr([]PhotoInput{{Key: "accounts/a/photos/1.jpg", Caption: " Pool "}}),
		IcalFeeds:       ptr([]IcalFeedInput{{Ref: "f", Channel: "airbnb", URL: "https://www.airbnb.com/calendar/ical/1.ics"}}),
	}
}

func TestValidPatchPasses(t *testing.T) {
	p := validPatch()
	p.Normalize()
	if err := p.Validate(); err != nil {
		t.Fatalf("valid patch: %v", err)
	}
	if (*p.Promos)[0].Code != "MONSOON-10" || p.Branding.Color != "#ff5a00" {
		t.Errorf("normalize: code %q, colour %q", (*p.Promos)[0].Code, p.Branding.Color)
	}
	if (*p.LengthDiscounts)[0].Nights != 7 || p.Payments.Bank.AccountNumber != "001234567890" {
		t.Errorf("normalize: discounts %v, account %q", *p.LengthDiscounts, p.Payments.Bank.AccountNumber)
	}
	if !p.TouchesPrices() {
		t.Error("patch with units should touch prices")
	}
}

func TestInvalidPatches(t *testing.T) {
	cases := map[string]struct {
		edit  func(p *PropertyPatch)
		field string
	}{
		"bad check-in":          {func(p *PropertyPatch) { p.CheckIn = ptr("2pm") }, "checkIn"},
		"policy":                {func(p *PropertyPatch) { p.Policy = ptr("lenient") }, "policy"},
		"max below min":         {func(p *PropertyPatch) { p.Rules.MaxNights = 1 }, "rules.maxNights"},
		"closed day":            {func(p *PropertyPatch) { p.Rules.ClosedArrival = []int{7} }, "rules.closedArrival"},
		"no display currency":   {func(p *PropertyPatch) { p.Booking.DisplayCurrencies = nil }, "booking.displayCurrencies"},
		"unknown currency":      {func(p *PropertyPatch) { p.Booking.DisplayCurrencies = []string{"JPY"} }, "booking.displayCurrencies"},
		"colour":                {func(p *PropertyPatch) { p.Branding.Color = "orange" }, "branding.color"},
		"bank without account":  {func(p *PropertyPatch) { p.Payments.Bank.AccountNumber = "" }, "payments.bank.accountNumber"},
		"bank without name":     {func(p *PropertyPatch) { p.Payments.Bank.BankName = "" }, "payments.bank.bankName"},
		"cards approved by app": {func(p *PropertyPatch) { p.Payments.Cards = "yes" }, "payments.cards"},
		"no units":              {func(p *PropertyPatch) { p.Units = ptr([]UnitInput{}) }, "units"},
		"zero rate":             {func(p *PropertyPatch) { (*p.Units)[0].Rate = 0 }, "units[0].rate"},
		"duplicate unit ref":    {func(p *PropertyPatch) { (*p.Units)[1].Ref = "ocean" }, "units[1].id"},
		"self link": {func(p *PropertyPatch) {
			(*p.Units)[2].LinkedUnitRef = []string{"whole"}
		}, "units[2].linkedUnitIds"},
		"nested whole house": {func(p *PropertyPatch) {
			(*p.Units)[0].LinkedUnitRef = []string{"whole"}
		}, "units[0].linkedUnitIds"},
		"unknown link": {func(p *PropertyPatch) {
			(*p.Units)[2].LinkedUnitRef = []string{"attic"}
		}, "units[2].linkedUnitIds"},
		"february 30":         {func(p *PropertyPatch) { (*p.Seasons)[0].End = "02-30" }, "seasons[0].end"},
		"discount too long":   {func(p *PropertyPatch) { (*p.LengthDiscounts)[0].Nights = 1 }, "lengthDiscounts[0].nights"},
		"same discount twice": {func(p *PropertyPatch) { (*p.LengthDiscounts)[1].Nights = 28 }, "lengthDiscounts[1].nights"},
		"percent charge over": {func(p *PropertyPatch) { (*p.Charges)[0].Amount = 120 }, "charges[0].amount"},
		"extra per":           {func(p *PropertyPatch) { (*p.Extras)[0].Per = "week" }, "extras[0].per"},
		"promo code":          {func(p *PropertyPatch) { (*p.Promos)[0].Code = "10% OFF" }, "promos[0].code"},
		"promo dates":         {func(p *PropertyPatch) { (*p.Promos)[0].To = "2026-04-01" }, "promos[0].to"},
		"duplicate promo": {func(p *PropertyPatch) {
			*p.Promos = append(*p.Promos, (*p.Promos)[0])
		}, "promos[1].code"},
		"duplicate photo": {func(p *PropertyPatch) {
			*p.Photos = append(*p.Photos, (*p.Photos)[0])
		}, "photos[1].key"},
		"feed url":      {func(p *PropertyPatch) { (*p.IcalFeeds)[0].URL = "airbnb calendar" }, "icalFeeds[0].url"},
		"extra guest 0": {func(p *PropertyPatch) { p.ExtraGuest.Above = 0 }, "extraGuest.above"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			p := validPatch()
			c.edit(&p)
			p.Normalize()
			err := p.Validate()
			var v *ValidationError
			if !errors.As(err, &v) {
				t.Fatalf("got %v, want a validation error on %s", err, c.field)
			}
			if v.Field != c.field {
				t.Errorf("field = %s (%s), want %s", v.Field, v.Message, c.field)
			}
		})
	}
}

func TestNewPropertyNeedsARateOrUnits(t *testing.T) {
	p := NewProperty{Name: "Kingfisher Villa", BookingType: BookingEntire, Currency: "USD"}
	p.Normalize()
	if err := p.Validate(); err == nil {
		t.Fatal("no rate and no units should fail")
	}
	p.Setup.Units = ptr([]UnitInput{{Ref: "u", Name: "Villa", Sleeps: 6, Rate: 18000}})
	if err := p.Validate(); err != nil {
		t.Fatalf("units without base rate: %v", err)
	}
}
