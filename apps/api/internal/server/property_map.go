package server

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	openapi_types "github.com/oapi-codegen/runtime/types"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/oapi"
)

// propertyToAPI turns a property's full setup into its API shape. Without the settings
// permission (caretakers), bank details and promo codes are left out.
func (s *Server) propertyToAPI(ctx context.Context, d domain.PropertyDetail, canSeeSettings bool) oapi.Property {
	out := oapi.Property{
		Id:              d.ID,
		Slug:            d.Slug,
		Name:            d.Name,
		BookingType:     oapi.BookingType(d.BookingType),
		Location:        optional(d.Location),
		Currency:        oapi.Currency(d.Currency),
		BaseRate:        d.BaseRateMinor,
		BookingPageUrl:  s.bookingPageURL(d.Slug),
		CreatedAt:       d.CreatedAt,
		Description:     d.Description,
		Amenities:       orEmpty(d.Amenities),
		CheckIn:         d.CheckIn,
		CheckOut:        d.CheckOut,
		TimeZone:        d.TimeZone,
		ExtraGuest:      oapi.ExtraGuest{Above: d.ExtraGuest.Above, Amount: d.ExtraGuest.Amount},
		Policy:          oapi.Policy(d.Policy),
		DepositPercent:  d.DepositPercent,
		BalanceDueDays:  d.BalanceDueDays,
		HouseRules:      orEmpty(d.HouseRules),
		Rules:           rulesToAPI(d.Rules),
		Booking:         bookingToAPI(d.Booking),
		Branding:        oapi.Branding{Color: d.Branding.Color},
		IcalExportToken: d.IcalExportToken,
		Photos:          make([]oapi.Photo, len(d.Photos)),
		Units:           make([]oapi.Unit, len(d.Units)),
		Seasons:         make([]oapi.Season, len(d.Seasons)),
		LengthDiscounts: make([]oapi.LengthDiscount, len(d.LengthDiscounts)),
		Charges:         make([]oapi.Charge, len(d.Charges)),
		Extras:          make([]oapi.Extra, len(d.Extras)),
		Promos:          make([]oapi.Promo, 0, len(d.Promos)),
		IcalFeeds:       make([]oapi.IcalFeed, len(d.IcalFeeds)),
		Payments: oapi.PaymentSettings{
			Bank: oapi.BankDetails{
				Enabled:        d.Payments.Bank.Enabled,
				BankName:       d.Payments.Bank.BankName,
				AccountName:    d.Payments.Bank.AccountName,
				AccountNumber:  d.Payments.Bank.AccountNumber,
				PayWithinHours: d.Payments.Bank.PayWithinHours,
				CancelIfUnpaid: d.Payments.Bank.CancelIfUnpaid,
			},
			AtProperty: d.Payments.AtProperty,
			Cards:      oapi.PaymentSettingsCards(d.Payments.Cards),
		},
	}
	if d.Branding.LogoKey != "" {
		key, url := d.Branding.LogoKey, s.files.URL(ctx, d.Branding.LogoKey)
		out.Branding.LogoKey, out.Branding.LogoUrl = &key, &url
	}
	for i, p := range d.Photos {
		out.Photos[i] = oapi.Photo{Id: p.ID, Key: p.Key, Url: s.files.URL(ctx, p.Key), Caption: p.Caption}
	}
	for i, u := range d.Units {
		out.Units[i] = oapi.Unit{
			Id: u.ID, Name: u.Name, Sleeps: u.Sleeps, Beds: u.Beds, Rate: u.Rate,
			LinkedUnitIds: orEmpty(u.LinkedUnitIDs),
		}
		if u.WeekendRate > 0 {
			w := u.WeekendRate
			out.Units[i].WeekendRate = &w
		}
	}
	for i, se := range d.Seasons {
		prices := make(map[string]int64, len(se.Prices))
		for unit, price := range se.Prices {
			prices[unit.String()] = price
		}
		out.Seasons[i] = oapi.Season{Id: se.ID, Name: se.Name, Start: se.Start, End: se.End, MinNights: optionalInt(se.MinNights), Prices: prices}
	}
	for i, ld := range d.LengthDiscounts {
		out.LengthDiscounts[i] = oapi.LengthDiscount{Nights: ld.Nights, Percent: ld.Percent}
	}
	for i, c := range d.Charges {
		out.Charges[i] = oapi.Charge{
			Id: c.ID, Name: c.Name, Kind: oapi.ChargeKind(c.Kind), Amount: c.Amount,
			Per: oapi.ChargePer(c.Per), Enabled: c.Enabled, Note: optional(c.Note),
		}
	}
	for i, e := range d.Extras {
		out.Extras[i] = oapi.Extra{
			Id: e.ID, Name: e.Name, Price: e.Price, Per: oapi.ExtraPer(e.Per), OnRequest: e.OnRequest, Enabled: e.Enabled,
		}
	}
	for i, f := range d.IcalFeeds {
		out.IcalFeeds[i] = oapi.IcalFeed{
			Id: f.ID, Channel: oapi.IcalChannel(f.Channel), Url: f.URL, Status: oapi.IcalFeedStatus(f.Status),
			LastSync: f.LastSync, Error: optional(f.Error), Upcoming: f.Upcoming,
		}
	}
	if !canSeeSettings {
		out.Payments.Bank.AccountName, out.Payments.Bank.AccountNumber = "", ""
		return out
	}
	for _, p := range d.Promos {
		out.Promos = append(out.Promos, oapi.Promo{
			Id: p.ID, Code: p.Code, Kind: oapi.ChargeKind(p.Kind), Amount: p.Amount,
			From: apiDay(p.From), To: apiDay(p.To), Limit: optionalInt(p.Limit), Used: p.Used,
			MinNights: optionalInt(p.MinNights), Note: optional(p.Note),
		})
	}
	return out
}

func rulesToAPI(r domain.StayRules) oapi.StayRules {
	return oapi.StayRules{
		MinNights: r.MinNights, MaxNights: r.MaxNights, SameDayCutoff: r.SameDayCutoff,
		WindowMonths: r.WindowMonths, ClosedArrival: orEmpty(r.ClosedArrival),
	}
}

func bookingToAPI(b domain.BookingSettings) oapi.BookingSettings {
	currencies := make([]oapi.BookingSettingsDisplayCurrencies, len(b.DisplayCurrencies))
	for i, c := range b.DisplayCurrencies {
		currencies[i] = oapi.BookingSettingsDisplayCurrencies(c)
	}
	return oapi.BookingSettings{
		Mode: oapi.BookingSettingsMode(b.Mode), ReplyHours: b.ReplyHours, HoldMinutes: b.HoldMinutes,
		DisplayCurrencies: currencies,
	}
}

// patchFromAPI turns an API patch into the domain's. Absent sections stay nil.
func patchFromAPI(in oapi.PropertyPatch) domain.PropertyPatch {
	p := domain.PropertyPatch{
		Name: in.Name, Description: in.Description, Location: in.Location,
		CheckIn: in.CheckIn, CheckOut: in.CheckOut, Amenities: in.Amenities,
		DepositPercent: in.DepositPercent, BalanceDueDays: in.BalanceDueDays, HouseRules: in.HouseRules,
	}
	if in.BookingType != nil {
		bt := domain.BookingType(*in.BookingType)
		p.BookingType = &bt
	}
	if in.Policy != nil {
		policy := string(*in.Policy)
		p.Policy = &policy
	}
	if r := in.Rules; r != nil {
		p.Rules = &domain.StayRules{
			MinNights: r.MinNights, MaxNights: r.MaxNights, SameDayCutoff: r.SameDayCutoff,
			WindowMonths: r.WindowMonths, ClosedArrival: orEmpty(r.ClosedArrival),
		}
	}
	if b := in.Booking; b != nil {
		currencies := make([]string, len(b.DisplayCurrencies))
		for i, c := range b.DisplayCurrencies {
			currencies[i] = string(c)
		}
		p.Booking = &domain.BookingSettings{Mode: string(b.Mode), ReplyHours: b.ReplyHours, HoldMinutes: b.HoldMinutes, DisplayCurrencies: currencies}
	}
	if b := in.Branding; b != nil {
		p.Branding = &domain.Branding{Color: b.Color, LogoKey: deref(b.LogoKey)}
	}
	if pay := in.Payments; pay != nil {
		p.Payments = &domain.PaymentSettings{
			Bank: domain.BankDetails{
				Enabled: pay.Bank.Enabled, BankName: pay.Bank.BankName, AccountName: pay.Bank.AccountName,
				AccountNumber: pay.Bank.AccountNumber, PayWithinHours: pay.Bank.PayWithinHours,
				CancelIfUnpaid: pay.Bank.CancelIfUnpaid,
			},
			AtProperty: pay.AtProperty,
			Cards:      string(pay.Cards),
		}
	}
	if g := in.ExtraGuest; g != nil {
		p.ExtraGuest = &domain.ExtraGuest{Above: g.Above, Amount: g.Amount}
	}
	if in.Units != nil {
		units := make([]domain.UnitInput, len(*in.Units))
		for i, u := range *in.Units {
			units[i] = domain.UnitInput{
				Ref: u.Id, Name: u.Name, Sleeps: u.Sleeps, Beds: deref(u.Beds), Rate: u.Rate,
				WeekendRate: deref(u.WeekendRate), LinkedUnitRef: derefSlice(u.LinkedUnitIds),
			}
		}
		p.Units = &units
	}
	if in.Seasons != nil {
		seasons := make([]domain.SeasonInput, len(*in.Seasons))
		for i, se := range *in.Seasons {
			seasons[i] = domain.SeasonInput{
				Ref: ref(se.Id, i), Name: se.Name, Start: se.Start, End: se.End,
				MinNights: deref(se.MinNights), Prices: se.Prices,
			}
		}
		p.Seasons = &seasons
	}
	if in.LengthDiscounts != nil {
		discounts := make([]domain.LengthDiscount, len(*in.LengthDiscounts))
		for i, d := range *in.LengthDiscounts {
			discounts[i] = domain.LengthDiscount{Nights: d.Nights, Percent: d.Percent}
		}
		p.LengthDiscounts = &discounts
	}
	if in.Charges != nil {
		charges := make([]domain.ChargeInput, len(*in.Charges))
		for i, c := range *in.Charges {
			charges[i] = domain.ChargeInput{Ref: ref(c.Id, i), Charge: domain.Charge{
				Name: c.Name, Kind: string(c.Kind), Amount: c.Amount, Per: string(c.Per), Enabled: c.Enabled, Note: deref(c.Note),
			}}
		}
		p.Charges = &charges
	}
	if in.Extras != nil {
		extras := make([]domain.ExtraInput, len(*in.Extras))
		for i, e := range *in.Extras {
			extras[i] = domain.ExtraInput{Ref: ref(e.Id, i), Extra: domain.Extra{
				Name: e.Name, Price: e.Price, Per: string(e.Per), OnRequest: e.OnRequest, Enabled: e.Enabled,
			}}
		}
		p.Extras = &extras
	}
	if in.Promos != nil {
		promos := make([]domain.PromoInput, len(*in.Promos))
		for i, pr := range *in.Promos {
			promos[i] = domain.PromoInput{Ref: ref(pr.Id, i), Promo: domain.Promo{
				Code: pr.Code, Kind: string(pr.Kind), Amount: pr.Amount, From: dayString(pr.From), To: dayString(pr.To),
				Limit: deref(pr.Limit), MinNights: deref(pr.MinNights), Note: deref(pr.Note),
			}}
		}
		p.Promos = &promos
	}
	if in.Photos != nil {
		photos := make([]domain.PhotoInput, len(*in.Photos))
		for i, ph := range *in.Photos {
			photos[i] = domain.PhotoInput{Key: ph.Key, Caption: deref(ph.Caption)}
		}
		p.Photos = &photos
	}
	if in.IcalFeeds != nil {
		feeds := make([]domain.IcalFeedInput, len(*in.IcalFeeds))
		for i, f := range *in.IcalFeeds {
			feeds[i] = domain.IcalFeedInput{Ref: ref(f.Id, i), Channel: string(f.Channel), URL: f.Url}
		}
		p.IcalFeeds = &feeds
	}
	return p
}

// ref is an item's id, or a placeholder that marks it as new.
func ref(id *string, i int) string {
	if id == nil || *id == "" {
		return "new-" + uuid.NewString() + "-" + string(rune('a'+i%26))
	}
	return *id
}

func deref[T any](v *T) T {
	var zero T
	if v == nil {
		return zero
	}
	return *v
}

func derefSlice[T any](v *[]T) []T {
	if v == nil {
		return nil
	}
	return *v
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func optionalInt(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func apiDay(s string) *openapi_types.Date {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &openapi_types.Date{Time: t}
}

func dayString(d *openapi_types.Date) string {
	if d == nil {
		return ""
	}
	return d.Format(time.DateOnly)
}

// fieldOf trims a domain field path to the top-level request field, for error codes.
func fieldOf(path string) string {
	if i := strings.IndexAny(path, ".["); i > 0 {
		return path[:i]
	}
	return path
}
