// Package pricing prices stays. It is the source of truth for totals; the owner app keeps a
// copy in apps/mobile/src/data/pricing.ts for instant previews, and both are held to the
// cases in packages/api-spec/fixtures/pricing.json.
package pricing

import (
	"fmt"
	"sort"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// Override is a manual change to one unit's night. Zero values mean no change.
type Override struct {
	Price           int64
	MinNights       int
	ClosedToArrival bool
}

// Overrides are a property's overrides by unit, then by night as "YYYY-MM-DD".
type Overrides map[uuid.UUID]map[string]Override

func (o Overrides) at(unitID uuid.UUID, night time.Time) Override {
	return o[unitID][night.Format(time.DateOnly)]
}

// Input describes the stay to price.
type Input struct {
	UnitID   uuid.UUID
	From     time.Time
	To       time.Time
	Adults   int
	Children int
	Extras   []uuid.UUID
	Promo    string
}

// Night is one night's price.
type Night struct {
	Night time.Time
	Price int64
}

// Quote is the guest's price breakdown, line by line, as the booking page shows it.
type Quote struct {
	Nights  int
	Nightly []Night
	Lines   []domain.PriceLine
	Extras  []domain.BookingExtra
	PromoID *uuid.UUID
	Total   int64
}

// NightlyRate is the price of one night of one unit: an override, then a season, then the
// unit's rate for that day of the week.
func NightlyRate(p domain.PropertyDetail, unitID uuid.UUID, night time.Time, o Overrides) int64 {
	if price := o.at(unitID, night).Price; price > 0 {
		return price
	}
	for _, s := range p.Seasons {
		if price, ok := s.Prices[unitID]; ok && inSeason(night, s.Start, s.End) {
			return price
		}
	}
	for _, u := range p.Units {
		if u.ID != unitID {
			continue
		}
		if wd := night.Weekday(); (wd == time.Friday || wd == time.Saturday) && u.WeekendRate > 0 {
			return u.WeekendRate
		}
		return u.Rate
	}
	return 0
}

// MinNights is the shortest stay allowed when arriving on checkIn.
func MinNights(p domain.PropertyDetail, unitID uuid.UUID, checkIn time.Time, o Overrides) int {
	if n := o.at(unitID, checkIn).MinNights; n > 0 {
		return n
	}
	for _, s := range p.Seasons {
		if s.MinNights > 0 && inSeason(checkIn, s.Start, s.End) {
			return s.MinNights
		}
	}
	return p.Rules.MinNights
}

// Price builds the quote for a stay.
func Price(p domain.PropertyDetail, in Input, o Overrides) Quote {
	q := Quote{Nights: max(0, domain.Nights(in.From, in.To))}
	for night := in.From; night.Before(in.To); night = night.AddDate(0, 0, 1) {
		q.Nightly = append(q.Nightly, Night{night, NightlyRate(p, in.UnitID, night, o)})
	}
	if q.Nights == 0 {
		return q
	}
	nights := int64(q.Nights)
	guests := int64(in.Adults + in.Children)

	var roomTotal int64
	for _, n := range q.Nightly {
		roomTotal += n.Price
	}
	q.add(domain.PriceLine{Label: plural(q.Nights, "night", "nights"), Amount: roomTotal})
	room := roomTotal

	discounts := append([]domain.LengthDiscount(nil), p.LengthDiscounts...)
	sort.SliceStable(discounts, func(i, j int) bool { return discounts[i].Nights > discounts[j].Nights })
	for _, d := range discounts {
		if q.Nights >= d.Nights {
			off := percentOf(roomTotal, int64(d.Percent))
			q.add(domain.PriceLine{Label: fmt.Sprintf("%d+ nights, %d%% off", d.Nights, d.Percent), Amount: -off, Kind: "discount"})
			room -= off
			break
		}
	}

	for _, promo := range p.Promos {
		if in.Promo == "" || promo.Code != in.Promo {
			continue
		}
		off := min(room, promo.Amount)
		if promo.Kind == "percent" {
			off = percentOf(room, promo.Amount)
		}
		q.add(domain.PriceLine{Label: "Code " + promo.Code, Amount: -off, Kind: "discount"})
		q.PromoID = &promo.ID
		room -= off
		break
	}

	if g := p.ExtraGuest; g.Amount > 0 && guests > int64(g.Above) {
		over := guests - int64(g.Above)
		amount := over * g.Amount * nights
		q.add(domain.PriceLine{Label: fmt.Sprintf("%d extra %s", over, pluralWord(int(over), "guest", "guests")), Amount: amount})
		room += amount
	}

	for _, id := range in.Extras {
		for _, e := range p.Extras {
			if e.ID != id {
				continue
			}
			amount := extraTotal(e, nights, guests)
			q.add(domain.PriceLine{Label: e.Name, Amount: amount, Kind: "extra"})
			q.Extras = append(q.Extras, domain.BookingExtra{ExtraID: e.ID, Name: e.Name, Amount: amount})
			break
		}
	}

	for _, c := range p.Charges {
		if !c.Enabled {
			continue
		}
		line := domain.PriceLine{Label: c.Name, Kind: "charge"}
		switch {
		case c.Kind == "percent":
			line.Label = fmt.Sprintf("%s %d%%", c.Name, c.Amount)
			line.Amount = percentOf(room, c.Amount)
		case c.Per == "night":
			line.Amount = c.Amount * nights
		case c.Per == "guest":
			line.Amount = c.Amount * guests
		default:
			line.Amount = c.Amount
		}
		q.add(line)
	}
	return q
}

// CustomLines is the breakdown for a price agreed with the guest instead of the quote.
func CustomLines(nights int, total int64) []domain.PriceLine {
	return []domain.PriceLine{{Label: plural(nights, "night", "nights") + ", agreed price", Amount: total}}
}

// Deposit is the share of a total the property asks for up front.
func Deposit(p domain.PropertyDetail, total int64) int64 {
	return percentOf(total, int64(p.DepositPercent))
}

// Refund is what the cancellation policy says to give back of what was paid, when the booking
// is cancelled on the day `on`.
func Refund(policy string, paid int64, checkIn, on time.Time) int64 {
	daysOut := domain.Nights(on, checkIn)
	var percent int64
	switch {
	case daysOut < 0:
	case policy == "flexible" && daysOut >= 7:
		percent = 100
	case policy == "moderate" && daysOut >= 14:
		percent = 100
	case policy == "moderate":
		percent = 50
	case policy == "strict" && daysOut >= 30:
		percent = 50
	}
	return percentOf(paid, percent)
}

func (q *Quote) add(l domain.PriceLine) {
	q.Lines = append(q.Lines, l)
	q.Total += l.Amount
}

func extraTotal(e domain.Extra, nights, guests int64) int64 {
	switch e.Per {
	case "night":
		return e.Price * nights
	case "guest":
		return e.Price * guests
	case "guestNight":
		return e.Price * guests * nights
	default: // stay, trip
		return e.Price
	}
}

// percentOf rounds half up like Math.round in the app. Amounts here are never negative.
func percentOf(amount, percent int64) int64 { return (amount*percent + 50) / 100 }

// inSeason reports whether night falls between two "MM-DD" days, which may wrap the new year.
func inSeason(night time.Time, start, end string) bool {
	md := night.Format("01-02")
	if start <= end {
		return md >= start && md <= end
	}
	return md >= start || md <= end
}

func plural(n int, one, many string) string { return fmt.Sprintf("%d %s", n, pluralWord(n, one, many)) }

func pluralWord(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
