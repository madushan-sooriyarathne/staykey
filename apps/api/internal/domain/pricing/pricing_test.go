package pricing

import (
	"encoding/json"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
)

// The fixture's properties are shaped like the owner app's PropertyConfig, so the same file
// drives src/data/__tests__/pricing-fixtures.test.ts.
type fixture struct {
	Properties map[string]struct {
		Units []struct {
			ID          uuid.UUID `json:"id"`
			Rate        int64     `json:"rate"`
			WeekendRate int64     `json:"weekendRate"`
		} `json:"units"`
		ExtraGuest *domain.ExtraGuest `json:"extraGuest"`
		Seasons    []struct {
			Start     string              `json:"start"`
			End       string              `json:"end"`
			Prices    map[uuid.UUID]int64 `json:"prices"`
			MinNights int                 `json:"minNights"`
		} `json:"seasons"`
		LengthDiscounts []domain.LengthDiscount `json:"lengthDiscounts"`
		Promos          []domain.Promo          `json:"promos"`
		Extras          []domain.Extra          `json:"extras"`
		Charges         []domain.Charge         `json:"charges"`
	} `json:"properties"`
	Cases []struct {
		Name     string `json:"name"`
		Property string `json:"property"`
		Input    struct {
			UnitID    uuid.UUID                                      `json:"unitId"`
			From      string                                         `json:"from"`
			To        string                                         `json:"to"`
			Adults    int                                            `json:"adults"`
			Children  int                                            `json:"children"`
			Extras    []uuid.UUID                                    `json:"extras"`
			Promo     string                                         `json:"promo"`
			Overrides map[uuid.UUID]map[string]struct{ Price int64 } `json:"overrides"`
		} `json:"input"`
		Expect result `json:"expect"`
	} `json:"cases"`
}

type result struct {
	Nights int `json:"nights"`
	Lines  []struct {
		Label  string `json:"label"`
		Amount int64  `json:"amount"`
		Kind   string `json:"kind"`
	} `json:"lines"`
	Total int64 `json:"total"`
}

func TestPriceMatchesSharedFixtures(t *testing.T) {
	raw, err := os.ReadFile("../../../../../packages/api-spec/fixtures/pricing.json")
	if err != nil {
		t.Fatal(err)
	}
	var f fixture
	if err := json.Unmarshal(raw, &f); err != nil {
		t.Fatal(err)
	}
	for _, c := range f.Cases {
		t.Run(c.Name, func(t *testing.T) {
			fp := f.Properties[c.Property]
			p := domain.PropertyDetail{LengthDiscounts: fp.LengthDiscounts, Promos: fp.Promos, Extras: fp.Extras, Charges: fp.Charges}
			for _, u := range fp.Units {
				p.Units = append(p.Units, domain.Unit{ID: u.ID, Rate: u.Rate, WeekendRate: u.WeekendRate})
			}
			for _, s := range fp.Seasons {
				p.Seasons = append(p.Seasons, domain.Season{Start: s.Start, End: s.End, Prices: s.Prices, MinNights: s.MinNights})
			}
			if fp.ExtraGuest != nil {
				p.ExtraGuest = *fp.ExtraGuest
			}
			o := Overrides{}
			for unit, nights := range c.Input.Overrides {
				o[unit] = map[string]Override{}
				for night, v := range nights {
					o[unit][night] = Override{Price: v.Price}
				}
			}

			q := Price(p, Input{
				UnitID: c.Input.UnitID, From: day(c.Input.From), To: day(c.Input.To),
				Adults: c.Input.Adults, Children: c.Input.Children, Extras: c.Input.Extras, Promo: c.Input.Promo,
			}, o)

			got := result{Nights: q.Nights, Total: q.Total, Lines: c.Expect.Lines[:0:0]}
			for _, l := range q.Lines {
				got.Lines = append(got.Lines, struct {
					Label  string `json:"label"`
					Amount int64  `json:"amount"`
					Kind   string `json:"kind"`
				}{l.Label, l.Amount, l.Kind})
			}
			if !reflect.DeepEqual(got, c.Expect) {
				t.Errorf("got %+v, want %+v", got, c.Expect)
			}
		})
	}
}

func TestMinNights(t *testing.T) {
	unit := uuid.New()
	p := domain.PropertyDetail{
		Rules:   domain.StayRules{MinNights: 2},
		Seasons: []domain.Season{{Start: "12-15", End: "01-15", MinNights: 5}},
	}
	tests := []struct {
		name      string
		checkIn   string
		overrides Overrides
		want      int
	}{
		{"property minimum outside seasons", "2026-06-01", nil, 2},
		{"season minimum after the new year", "2027-01-10", nil, 5},
		{"override wins over the season", "2026-12-20", Overrides{unit: {"2026-12-20": {MinNights: 1}}}, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := MinNights(p, unit, day(tt.checkIn), tt.overrides); got != tt.want {
				t.Errorf("MinNights = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestRefund(t *testing.T) {
	checkIn := day("2026-07-30")
	tests := []struct {
		policy string
		on     string
		want   int64
	}{
		{"flexible", "2026-07-23", 10001},
		{"flexible", "2026-07-24", 0},
		{"moderate", "2026-07-16", 10001},
		{"moderate", "2026-07-17", 5001},
		{"strict", "2026-06-30", 5001},
		{"strict", "2026-07-01", 0},
		{"moderate", "2026-07-31", 0},
	}
	for _, tt := range tests {
		t.Run(tt.policy+" on "+tt.on, func(t *testing.T) {
			if got := Refund(tt.policy, 10001, checkIn, day(tt.on)); got != tt.want {
				t.Errorf("Refund = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestDepositRoundsHalfUp(t *testing.T) {
	if got := Deposit(domain.PropertyDetail{DepositPercent: 30}, 1005); got != 302 {
		t.Errorf("Deposit = %d, want 302", got)
	}
}

func day(s string) time.Time {
	d, err := time.Parse(time.DateOnly, s)
	if err != nil {
		panic(err)
	}
	return d
}
