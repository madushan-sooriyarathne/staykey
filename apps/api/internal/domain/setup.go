package domain

import (
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
)

// A property's full setup, as the owner app edits it. Money is in minor units of the property's
// currency, percents are whole numbers, and calendar days are "YYYY-MM-DD".

// Unit is something guests book: the whole place, or one room.
type Unit struct {
	ID          uuid.UUID
	Name        string
	Sleeps      int
	Beds        string
	Rate        int64 // Sunday to Thursday
	WeekendRate int64 // Friday and Saturday; 0 means the same as Rate
	// LinkedUnitIDs makes this a whole-house unit that books these rooms together.
	LinkedUnitIDs []uuid.UUID
}

// Photo is an uploaded picture of the property. The first is the cover.
type Photo struct {
	ID      uuid.UUID
	Key     string
	Caption string
}

// ExtraGuest charges per guest per night above a number of included guests. Amount 0 is off.
type ExtraGuest struct {
	Above  int
	Amount int64
}

// StayRules limit which stays can be booked.
type StayRules struct {
	MinNights     int
	MaxNights     int
	SameDayCutoff *int // hour until which same-day arrivals can book; nil turns them off
	WindowMonths  int
	ClosedArrival []int // weekdays, 0 for Sunday
}

// BookingSettings decide how guests book.
type BookingSettings struct {
	Mode              string // instant or request
	ReplyHours        int
	HoldMinutes       int
	DisplayCurrencies []string
}

// Branding styles the booking page.
type Branding struct {
	Color   string // "#rrggbb"
	LogoKey string
}

// BankDetails is how guests pay by bank transfer.
type BankDetails struct {
	Enabled        bool
	BankName       string
	AccountName    string
	AccountNumber  string
	PayWithinHours int
	CancelIfUnpaid bool
}

// PaymentSettings lists the ways guests can pay.
type PaymentSettings struct {
	Bank       BankDetails
	AtProperty bool
	Cards      string // off, pending (applied) or on (approved by PayHere)
}

// Season changes nightly prices between two "MM-DD" days every year.
type Season struct {
	ID        uuid.UUID
	Name      string
	Start     string
	End       string
	MinNights int // 0 means the property's minimum
	Prices    map[uuid.UUID]int64
}

// LengthDiscount takes a percent off stays of at least Nights.
type LengthDiscount struct {
	Nights  int
	Percent int
}

// Charge is a service charge, tax or fee added to every booking.
type Charge struct {
	ID      uuid.UUID
	Name    string
	Kind    string // percent or fixed
	Amount  int64
	Per     string // stay, night or guest
	Enabled bool
	Note    string
}

// Extra is something guests can add: a transfer, breakfast, a tour.
type Extra struct {
	ID        uuid.UUID
	Name      string
	Price     int64
	Per       string // stay, night, guest, trip or guestNight
	OnRequest bool
	Enabled   bool
}

// Promo is a discount code.
type Promo struct {
	ID        uuid.UUID
	Code      string
	Kind      string // percent or fixed
	Amount    int64
	From      string // "YYYY-MM-DD" or empty
	To        string
	Limit     int // 0 means unlimited
	Used      int
	MinNights int
	Note      string
}

// IcalFeed is an OTA calendar imported into the property's calendar.
type IcalFeed struct {
	ID       uuid.UUID
	Channel  string
	URL      string
	Status   string // pending, ok or error
	LastSync *time.Time
	Error    string
	Upcoming int
}

// PropertyDetail is a property with its full setup.
type PropertyDetail struct {
	Property
	Description     string
	Amenities       []string
	CheckIn         string
	CheckOut        string
	TimeZone        string
	Photos          []Photo
	Units           []Unit
	ExtraGuest      ExtraGuest
	Seasons         []Season
	LengthDiscounts []LengthDiscount
	Rules           StayRules
	Policy          string
	DepositPercent  int
	BalanceDueDays  int
	HouseRules      []string
	Charges         []Charge
	Extras          []Extra
	Promos          []Promo
	Payments        PaymentSettings
	Booking         BookingSettings
	Branding        Branding
	IcalFeeds       []IcalFeed
	IcalExportToken string
}

// Inputs refer to collection items by Ref: the id of an existing item, or any other string for
// a new one. New units can be referenced by their Ref from links and season prices in the same
// patch.

// UnitInput adds or changes a unit.
type UnitInput struct {
	Ref           string
	Name          string
	Sleeps        int
	Beds          string
	Rate          int64
	WeekendRate   int64
	LinkedUnitRef []string
}

// SeasonInput adds or changes a season. Prices are keyed by unit Ref.
type SeasonInput struct {
	Ref       string
	Name      string
	Start     string
	End       string
	MinNights int
	Prices    map[string]int64
}

// ChargeInput adds or changes a charge.
type ChargeInput struct {
	Ref string
	Charge
}

// ExtraInput adds or changes an extra.
type ExtraInput struct {
	Ref string
	Extra
}

// PromoInput adds or changes a promo code. Used is kept by the server.
type PromoInput struct {
	Ref string
	Promo
}

// IcalFeedInput adds or changes an imported calendar. Status is kept by the server.
type IcalFeedInput struct {
	Ref     string
	Channel string
	URL     string
}

// PhotoInput lists an uploaded photo, in display order.
type PhotoInput struct {
	Key     string
	Caption string
}

// PropertyPatch changes some of a property's setup. Nil fields stay as they are; collections
// that are present replace the whole collection.
type PropertyPatch struct {
	Name           *string
	BookingType    *BookingType
	Description    *string
	Location       *string
	CheckIn        *string
	CheckOut       *string
	Amenities      *[]string
	Policy         *string
	DepositPercent *int
	BalanceDueDays *int
	HouseRules     *[]string
	Rules          *StayRules
	Booking        *BookingSettings
	Branding       *Branding
	Payments       *PaymentSettings
	ExtraGuest     *ExtraGuest

	Units           *[]UnitInput
	Seasons         *[]SeasonInput
	LengthDiscounts *[]LengthDiscount
	Charges         *[]ChargeInput
	Extras          *[]ExtraInput
	Promos          *[]PromoInput
	Photos          *[]PhotoInput
	IcalFeeds       *[]IcalFeedInput
}

// TouchesPrices reports whether the patch changes money settings, which need the prices
// permission as well as settings.
func (p PropertyPatch) TouchesPrices() bool {
	return p.Units != nil || p.Seasons != nil || p.LengthDiscounts != nil || p.Charges != nil ||
		p.Extras != nil || p.Promos != nil || p.ExtraGuest != nil
}

// Limits on a property's setup.
const (
	MaxUnits      = 20
	MaxPhotos     = 30
	MaxSeasons    = 20
	MaxCharges    = 10
	MaxExtras     = 30
	MaxPromos     = 50
	MaxIcalFeeds  = 10
	MaxMoneyMinor = 1_000_000_000_00 // a billion in major units; keeps totals far from overflow
)

var (
	clockPattern    = regexp.MustCompile(`^([01][0-9]|2[0-3]):[0-5][0-9]$`)
	monthDayPattern = regexp.MustCompile(`^(0[1-9]|1[0-2])-(0[1-9]|[12][0-9]|3[01])$`)
	colorPattern    = regexp.MustCompile(`^#[0-9a-f]{6}$`)
	promoPattern    = regexp.MustCompile(`^[A-Z0-9-]{3,20}$`)

	policies           = []string{"flexible", "moderate", "strict"}
	bookingModes       = []string{"instant", "request"}
	displayCurrencies  = []string{"USD", "EUR", "GBP", "AUD", "INR", "LKR"}
	chargeKinds        = []string{"percent", "fixed"}
	chargePers         = []string{"stay", "night", "guest"}
	extraPers          = []string{"stay", "night", "guest", "trip", "guestNight"}
	icalChannels       = []string{"airbnb", "booking", "agoda", "expedia", "other"}
	cardsRequestable   = []string{"off", "pending"}
	daysInMonth        = [13]int{0, 31, 29, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31}
	defaultHouseRules  = []string{"No smoking indoors", "No parties or events"}
	defaultBrandColor  = "#09090b"
	defaultCheckIn     = "14:00"
	defaultCheckOut    = "11:00"
	defaultSameDayHour = 12
)

// DefaultCharges are offered, switched off, on every new property.
func DefaultCharges() []ChargeInput {
	return []ChargeInput{
		{Ref: "service", Charge: Charge{Name: "Service charge", Kind: "percent", Amount: 10, Per: "stay", Note: "on the room rate"}},
		{Ref: "vat", Charge: Charge{Name: "VAT", Kind: "percent", Amount: 18, Per: "stay", Note: "on the room rate, once you register"}},
	}
}

// DefaultSettings is the setup a new property starts with, before anything the owner sends.
type DefaultSettings struct {
	CheckIn, CheckOut string
	HouseRules        []string
	Rules             StayRules
	Booking           BookingSettings
	BrandColor        string
}

// Defaults returns the starting setup for a property charging in currency.
func Defaults(currency string) DefaultSettings {
	cutoff := defaultSameDayHour
	return DefaultSettings{
		CheckIn:    defaultCheckIn,
		CheckOut:   defaultCheckOut,
		HouseRules: slices.Clone(defaultHouseRules),
		Rules:      StayRules{MinNights: 1, MaxNights: 30, SameDayCutoff: &cutoff, WindowMonths: 12, ClosedArrival: []int{}},
		Booking:    BookingSettings{Mode: "instant", ReplyHours: 24, HoldMinutes: 15, DisplayCurrencies: []string{currency}},
		BrandColor: defaultBrandColor,
	}
}

// Normalize trims text, upper-cases promo codes and drops duplicates.
func (p *PropertyPatch) Normalize() {
	trim := func(s *string) {
		if s != nil {
			*s = strings.Join(strings.Fields(*s), " ")
		}
	}
	trim(p.Name)
	trim(p.Location)
	if p.Description != nil {
		*p.Description = strings.TrimSpace(*p.Description)
	}
	if p.Amenities != nil {
		*p.Amenities = cleanList(*p.Amenities)
	}
	if p.HouseRules != nil {
		*p.HouseRules = cleanList(*p.HouseRules)
	}
	if p.Rules != nil {
		slices.Sort(p.Rules.ClosedArrival)
		p.Rules.ClosedArrival = slices.Compact(p.Rules.ClosedArrival)
	}
	if p.Booking != nil {
		for i, c := range p.Booking.DisplayCurrencies {
			p.Booking.DisplayCurrencies[i] = strings.ToUpper(strings.TrimSpace(c))
		}
		p.Booking.DisplayCurrencies = cleanList(p.Booking.DisplayCurrencies)
	}
	if p.Branding != nil {
		p.Branding.Color = strings.ToLower(strings.TrimSpace(p.Branding.Color))
	}
	if p.Payments != nil {
		b := &p.Payments.Bank
		b.BankName = strings.TrimSpace(b.BankName)
		b.AccountName = strings.Join(strings.Fields(b.AccountName), " ")
		b.AccountNumber = strings.Join(strings.Fields(b.AccountNumber), "")
	}
	if p.Units != nil {
		for i := range *p.Units {
			u := &(*p.Units)[i]
			trim(&u.Name)
			trim(&u.Beds)
		}
	}
	if p.Seasons != nil {
		for i := range *p.Seasons {
			trim(&(*p.Seasons)[i].Name)
		}
	}
	if p.LengthDiscounts != nil {
		slices.SortFunc(*p.LengthDiscounts, func(a, b LengthDiscount) int { return a.Nights - b.Nights })
	}
	if p.Charges != nil {
		for i := range *p.Charges {
			c := &(*p.Charges)[i]
			trim(&c.Name)
			trim(&c.Note)
		}
	}
	if p.Extras != nil {
		for i := range *p.Extras {
			trim(&(*p.Extras)[i].Name)
		}
	}
	if p.Promos != nil {
		for i := range *p.Promos {
			pr := &(*p.Promos)[i]
			pr.Code = strings.ToUpper(strings.TrimSpace(pr.Code))
			trim(&pr.Note)
		}
	}
	if p.Photos != nil {
		for i := range *p.Photos {
			trim(&(*p.Photos)[i].Caption)
		}
	}
	if p.IcalFeeds != nil {
		for i := range *p.IcalFeeds {
			(*p.IcalFeeds)[i].URL = strings.TrimSpace((*p.IcalFeeds)[i].URL)
		}
	}
}

// Validate checks every section the patch carries. Call Normalize first. References between
// collections (season prices and links to units) are checked when the patch is applied.
func (p PropertyPatch) Validate() error {
	checks := []func() error{
		p.validateDetails, p.validatePolicies, p.validateRules, p.validateBooking,
		p.validateBranding, p.validatePayments, p.validateUnits, p.validatePricing,
		p.validatePromos, p.validatePhotos, p.validateFeeds,
	}
	for _, check := range checks {
		if err := check(); err != nil {
			return err
		}
	}
	return nil
}

func invalid(field, format string, args ...any) error {
	return &ValidationError{Field: field, Message: fmt.Sprintf(format, args...)}
}

func (p PropertyPatch) validateDetails() error {
	if p.Name != nil && (len(*p.Name) < 2 || len(*p.Name) > 80) {
		return invalid("name", "must be between 2 and 80 characters")
	}
	if p.BookingType != nil && !p.BookingType.Valid() {
		return invalid("bookingType", `must be "entire" or "rooms"`)
	}
	if p.Description != nil && len([]rune(*p.Description)) > 2000 {
		return invalid("description", "must be at most 2000 characters")
	}
	if p.Location != nil && len(*p.Location) > 160 {
		return invalid("location", "must be at most 160 characters")
	}
	for field, v := range map[string]*string{"checkIn": p.CheckIn, "checkOut": p.CheckOut} {
		if v != nil && !clockPattern.MatchString(*v) {
			return invalid(field, "must be a time like 14:00")
		}
	}
	if p.Amenities != nil {
		if len(*p.Amenities) > 40 {
			return invalid("amenities", "can list at most 40 amenities")
		}
		for _, a := range *p.Amenities {
			if len(a) > 40 {
				return invalid("amenities", "each amenity must be at most 40 characters")
			}
		}
	}
	return nil
}

func (p PropertyPatch) validatePolicies() error {
	if p.Policy != nil && !slices.Contains(policies, *p.Policy) {
		return invalid("policy", "must be flexible, moderate or strict")
	}
	if p.DepositPercent != nil && (*p.DepositPercent < 0 || *p.DepositPercent > 100) {
		return invalid("depositPercent", "must be between 0 and 100")
	}
	if p.BalanceDueDays != nil && (*p.BalanceDueDays < 0 || *p.BalanceDueDays > 90) {
		return invalid("balanceDueDays", "must be between 0 and 90")
	}
	if p.HouseRules != nil {
		if len(*p.HouseRules) > 20 {
			return invalid("houseRules", "can list at most 20 rules")
		}
		for _, r := range *p.HouseRules {
			if len(r) > 120 {
				return invalid("houseRules", "each rule must be at most 120 characters")
			}
		}
	}
	return nil
}

func (p PropertyPatch) validateRules() error {
	r := p.Rules
	if r == nil {
		return nil
	}
	switch {
	case r.MinNights < 1 || r.MinNights > 30:
		return invalid("rules.minNights", "must be between 1 and 30")
	case r.MaxNights < r.MinNights || r.MaxNights > 365:
		return invalid("rules.maxNights", "must be between the minimum and 365")
	case r.SameDayCutoff != nil && (*r.SameDayCutoff < 0 || *r.SameDayCutoff > 23):
		return invalid("rules.sameDayCutoff", "must be an hour between 0 and 23")
	case r.WindowMonths < 1 || r.WindowMonths > 24:
		return invalid("rules.windowMonths", "must be between 1 and 24")
	}
	for _, d := range r.ClosedArrival {
		if d < 0 || d > 6 {
			return invalid("rules.closedArrival", "days must be 0 (Sunday) to 6 (Saturday)")
		}
	}
	return nil
}

func (p PropertyPatch) validateBooking() error {
	b := p.Booking
	if b == nil {
		return nil
	}
	switch {
	case !slices.Contains(bookingModes, b.Mode):
		return invalid("booking.mode", "must be instant or request")
	case b.ReplyHours < 1 || b.ReplyHours > 72:
		return invalid("booking.replyHours", "must be between 1 and 72")
	case b.HoldMinutes < 5 || b.HoldMinutes > 120:
		return invalid("booking.holdMinutes", "must be between 5 and 120")
	case len(b.DisplayCurrencies) == 0:
		return invalid("booking.displayCurrencies", "pick at least one currency")
	}
	for _, c := range b.DisplayCurrencies {
		if !slices.Contains(displayCurrencies, c) {
			return invalid("booking.displayCurrencies", "must be from %s", strings.Join(displayCurrencies, ", "))
		}
	}
	return nil
}

func (p PropertyPatch) validateBranding() error {
	if p.Branding != nil && !colorPattern.MatchString(p.Branding.Color) {
		return invalid("branding.color", "must be a colour like #ff5a00")
	}
	return nil
}

func (p PropertyPatch) validatePayments() error {
	pay := p.Payments
	if pay == nil {
		return nil
	}
	b := pay.Bank
	if b.Enabled {
		switch {
		case b.BankName == "" || len(b.BankName) > 80:
			return invalid("payments.bank.bankName", "choose the bank guests pay into")
		case len(b.AccountName) < 2 || len(b.AccountName) > 80:
			return invalid("payments.bank.accountName", "enter the name on the account")
		case !validAccountNumber(b.AccountNumber):
			return invalid("payments.bank.accountNumber", "enter an account number of 6 to 20 digits")
		}
	} else if b.AccountNumber != "" && !validAccountNumber(b.AccountNumber) {
		return invalid("payments.bank.accountNumber", "enter an account number of 6 to 20 digits")
	}
	if b.PayWithinHours < 1 || b.PayWithinHours > 168 {
		return invalid("payments.bank.payWithinHours", "must be between 1 and 168")
	}
	if !slices.Contains(cardsRequestable, pay.Cards) && pay.Cards != "on" {
		return invalid("payments.cards", "must be off or pending")
	}
	return nil
}

func validAccountNumber(s string) bool {
	if len(s) < 6 || len(s) > 20 {
		return false
	}
	for _, r := range s {
		if (r < '0' || r > '9') && r != '-' {
			return false
		}
	}
	return true
}

func money(field string, v int64, allowZero bool) error {
	if v < 0 || (!allowZero && v == 0) || v > MaxMoneyMinor {
		if allowZero {
			return invalid(field, "must be zero or more")
		}
		return invalid(field, "must be greater than zero")
	}
	return nil
}

func (p PropertyPatch) validateUnits() error {
	if p.Units == nil {
		return nil
	}
	units := *p.Units
	if len(units) == 0 {
		return invalid("units", "a property needs at least one unit")
	}
	if len(units) > MaxUnits {
		return invalid("units", "a property can have at most %d units", MaxUnits)
	}
	refs := map[string]bool{}
	linked := map[string]bool{}
	for i, u := range units {
		f := fmt.Sprintf("units[%d]", i)
		switch {
		case u.Ref == "":
			return invalid(f+".id", "is required, any unique text for new units")
		case refs[u.Ref]:
			return invalid(f+".id", "is used by another unit")
		case u.Name == "" || len(u.Name) > 60:
			return invalid(f+".name", "must be between 1 and 60 characters")
		case u.Sleeps < 1 || u.Sleeps > 30:
			return invalid(f+".sleeps", "must be between 1 and 30")
		case len(u.Beds) > 80:
			return invalid(f+".beds", "must be at most 80 characters")
		}
		if err := money(f+".rate", u.Rate, false); err != nil {
			return err
		}
		if err := money(f+".weekendRate", u.WeekendRate, true); err != nil {
			return err
		}
		refs[u.Ref] = true
		if len(u.LinkedUnitRef) > 0 {
			linked[u.Ref] = true
		}
	}
	for i, u := range units {
		for _, ref := range u.LinkedUnitRef {
			f := fmt.Sprintf("units[%d].linkedUnitIds", i)
			switch {
			case ref == u.Ref:
				return invalid(f, "a unit cannot include itself")
			case !refs[ref]:
				return invalid(f, "must list units of this property")
			case linked[ref]:
				return invalid(f, "cannot include another whole-house unit")
			}
		}
	}
	return nil
}

func (p PropertyPatch) validatePricing() error {
	if g := p.ExtraGuest; g != nil {
		if g.Above < 0 || g.Above > 30 {
			return invalid("extraGuest.above", "must be between 1 and 30")
		}
		if err := money("extraGuest.amount", g.Amount, true); err != nil {
			return err
		}
		if g.Amount > 0 && g.Above == 0 {
			return invalid("extraGuest.above", "must be between 1 and 30")
		}
	}
	if p.Seasons != nil {
		if len(*p.Seasons) > MaxSeasons {
			return invalid("seasons", "can have at most %d seasons", MaxSeasons)
		}
		for i, s := range *p.Seasons {
			f := fmt.Sprintf("seasons[%d]", i)
			switch {
			case s.Name == "" || len(s.Name) > 60:
				return invalid(f+".name", "must be between 1 and 60 characters")
			case !validMonthDay(s.Start):
				return invalid(f+".start", "must be a day like 12-15")
			case !validMonthDay(s.End):
				return invalid(f+".end", "must be a day like 01-15")
			case s.MinNights < 0 || s.MinNights > 30:
				return invalid(f+".minNights", "must be between 1 and 30")
			}
			for ref, price := range s.Prices {
				if err := money(f+".prices."+ref, price, false); err != nil {
					return err
				}
			}
		}
	}
	if p.LengthDiscounts != nil {
		seen := map[int]bool{}
		for i, d := range *p.LengthDiscounts {
			f := fmt.Sprintf("lengthDiscounts[%d]", i)
			switch {
			case d.Nights < 2 || d.Nights > 365:
				return invalid(f+".nights", "must be between 2 and 365")
			case seen[d.Nights]:
				return invalid(f+".nights", "has another discount for the same length")
			case d.Percent < 1 || d.Percent > 90:
				return invalid(f+".percent", "must be between 1 and 90")
			}
			seen[d.Nights] = true
		}
	}
	if p.Charges != nil {
		if len(*p.Charges) > MaxCharges {
			return invalid("charges", "can have at most %d charges", MaxCharges)
		}
		for i, c := range *p.Charges {
			f := fmt.Sprintf("charges[%d]", i)
			switch {
			case c.Name == "" || len(c.Name) > 60:
				return invalid(f+".name", "must be between 1 and 60 characters")
			case !slices.Contains(chargeKinds, c.Kind):
				return invalid(f+".kind", "must be percent or fixed")
			case !slices.Contains(chargePers, c.Per):
				return invalid(f+".per", "must be stay, night or guest")
			case c.Kind == "percent" && (c.Amount < 0 || c.Amount > 100):
				return invalid(f+".amount", "must be between 0 and 100 percent")
			case len(c.Note) > 120:
				return invalid(f+".note", "must be at most 120 characters")
			}
			if err := money(f+".amount", c.Amount, true); err != nil {
				return err
			}
		}
	}
	if p.Extras != nil {
		if len(*p.Extras) > MaxExtras {
			return invalid("extras", "can have at most %d extras", MaxExtras)
		}
		for i, e := range *p.Extras {
			f := fmt.Sprintf("extras[%d]", i)
			switch {
			case e.Name == "" || len(e.Name) > 60:
				return invalid(f+".name", "must be between 1 and 60 characters")
			case !slices.Contains(extraPers, e.Per):
				return invalid(f+".per", "must be stay, night, guest, trip or guestNight")
			}
			if err := money(f+".price", e.Price, true); err != nil {
				return err
			}
		}
	}
	return nil
}

func (p PropertyPatch) validatePromos() error {
	if p.Promos == nil {
		return nil
	}
	if len(*p.Promos) > MaxPromos {
		return invalid("promos", "can have at most %d codes", MaxPromos)
	}
	codes := map[string]bool{}
	for i, pr := range *p.Promos {
		f := fmt.Sprintf("promos[%d]", i)
		switch {
		case !promoPattern.MatchString(pr.Code):
			return invalid(f+".code", "use 3 to 20 letters, numbers or dashes")
		case codes[pr.Code]:
			return invalid(f+".code", "is already used by another code")
		case !slices.Contains(chargeKinds, pr.Kind):
			return invalid(f+".kind", "must be percent or fixed")
		case pr.Kind == "percent" && (pr.Amount < 1 || pr.Amount > 100):
			return invalid(f+".amount", "must be between 1 and 100 percent")
		case pr.From != "" && !validDate(pr.From):
			return invalid(f+".from", "must be a date like 2026-12-01")
		case pr.To != "" && !validDate(pr.To):
			return invalid(f+".to", "must be a date like 2026-12-31")
		case pr.From != "" && pr.To != "" && pr.To < pr.From:
			return invalid(f+".to", "must be on or after the start date")
		case pr.Limit < 0:
			return invalid(f+".limit", "must be 1 or more")
		case pr.MinNights < 0 || pr.MinNights > 365:
			return invalid(f+".minNights", "must be between 1 and 365")
		case len(pr.Note) > 120:
			return invalid(f+".note", "must be at most 120 characters")
		}
		if err := money(f+".amount", pr.Amount, false); err != nil {
			return err
		}
		codes[pr.Code] = true
	}
	return nil
}

func (p PropertyPatch) validatePhotos() error {
	if p.Photos == nil {
		return nil
	}
	if len(*p.Photos) > MaxPhotos {
		return invalid("photos", "can have at most %d photos", MaxPhotos)
	}
	seen := map[string]bool{}
	for i, ph := range *p.Photos {
		f := fmt.Sprintf("photos[%d]", i)
		switch {
		case ph.Key == "":
			return invalid(f+".key", "is required")
		case seen[ph.Key]:
			return invalid(f+".key", "is listed twice")
		case len(ph.Caption) > 120:
			return invalid(f+".caption", "must be at most 120 characters")
		}
		seen[ph.Key] = true
	}
	return nil
}

func (p PropertyPatch) validateFeeds() error {
	if p.IcalFeeds == nil {
		return nil
	}
	if len(*p.IcalFeeds) > MaxIcalFeeds {
		return invalid("icalFeeds", "can have at most %d calendars", MaxIcalFeeds)
	}
	for i, f := range *p.IcalFeeds {
		field := fmt.Sprintf("icalFeeds[%d]", i)
		if !slices.Contains(icalChannels, f.Channel) {
			return invalid(field+".channel", "must be airbnb, booking, agoda, expedia or other")
		}
		if f.URL != "" && !validFeedURL(f.URL) {
			return invalid(field+".url", "that link doesn't look like a calendar")
		}
	}
	return nil
}

func validMonthDay(s string) bool {
	if !monthDayPattern.MatchString(s) {
		return false
	}
	var m, d int
	_, _ = fmt.Sscanf(s, "%d-%d", &m, &d)
	return d <= daysInMonth[m]
}

func validDate(s string) bool {
	_, err := time.Parse(time.DateOnly, s)
	return err == nil
}

func validFeedURL(s string) bool {
	if len(s) > 1000 {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http" && u.Scheme != "webcal") || u.Host == "" {
		return false
	}
	return true
}

func cleanList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.Join(strings.Fields(s), " ")
		if s != "" && !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	return out
}

// RefID reads an input Ref as an existing id. ok is false for refs that name new items.
func RefID(ref string, existing map[uuid.UUID]bool) (uuid.UUID, bool) {
	id, err := uuid.Parse(ref)
	if err != nil || !existing[id] {
		return uuid.Nil, false
	}
	return id, true
}
