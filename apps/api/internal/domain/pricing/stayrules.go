package pricing

import (
	"fmt"
	"slices"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// GuestStay checks a stay a guest asks for on the booking page against the property's rules.
// Owners may book outside them from the app; guests may not. It lives here rather than in
// domain because it needs Overrides and MinNights, and this package already imports domain.
// Stay dates are local calendar days at UTC midnight; now is turned into the property's day and
// hour.
func GuestStay(p domain.PropertyDetail, in domain.StayInput, o Overrides, now time.Time) error {
	loc, err := time.LoadLocation(p.TimeZone)
	if err != nil {
		loc = time.UTC
	}
	local := now.In(loc)
	today := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, time.UTC)

	switch {
	case in.CheckIn.Before(today):
		return invalid("checkIn", "can't be in the past")
	case in.CheckIn.Equal(today) && (p.Rules.SameDayCutoff == nil || local.Hour() >= *p.Rules.SameDayCutoff):
		return invalid("checkIn", "same-day arrivals can't be booked online any more today")
	}
	if months := p.Rules.WindowMonths; months > 0 && in.CheckOut.After(today.AddDate(0, months, 0)) {
		return invalid("checkOut", fmt.Sprintf("must be within %d months", months))
	}

	nights := domain.Nights(in.CheckIn, in.CheckOut)
	if least := MinNights(p, in.UnitID, in.CheckIn, o); nights < least {
		return invalid("checkOut", fmt.Sprintf("stays arriving then are at least %s", plural(least, "night", "nights")))
	}
	if most := p.Rules.MaxNights; most > 0 && nights > most {
		return invalid("checkOut", fmt.Sprintf("stays are at most %s", plural(most, "night", "nights")))
	}

	if slices.Contains(p.Rules.ClosedArrival, int(in.CheckIn.Weekday())) || o.at(in.UnitID, in.CheckIn).ClosedToArrival {
		return invalid("checkIn", "guests can't arrive on that day")
	}

	unit, ok := findUnit(p.Units, in.UnitID)
	if !ok {
		return invalid("unitId", "must be a unit of this property")
	}
	// ExtraGuest only adds a charge above a number of guests; Sleeps is the real limit.
	if guests := in.Adults + in.Children; guests > unit.Sleeps {
		return invalid("adults", fmt.Sprintf("%s sleeps up to %d", unit.Name, unit.Sleeps))
	}
	return nil
}

// PromoProblem reports why a promo code can't be used on a stay, or nil when it can. Limit 0
// means unlimited.
func PromoProblem(promo domain.Promo, nights int, checkIn time.Time) error {
	day := checkIn.Format(time.DateOnly)
	switch {
	case promo.From != "" && day < promo.From, promo.To != "" && day > promo.To:
		return invalid("promo", "that code isn't valid for these dates")
	case promo.Limit > 0 && promo.Used >= promo.Limit:
		return invalid("promo", "that code has been used up")
	case promo.MinNights > 0 && nights < promo.MinNights:
		return invalid("promo", fmt.Sprintf("that code needs a stay of at least %s", plural(promo.MinNights, "night", "nights")))
	}
	return nil
}

func findUnit(units []domain.Unit, id uuid.UUID) (domain.Unit, bool) {
	for _, u := range units {
		if u.ID == id {
			return u, true
		}
	}
	return domain.Unit{}, false
}

func invalid(field, message string) error {
	return &domain.ValidationError{Field: field, Message: message}
}
