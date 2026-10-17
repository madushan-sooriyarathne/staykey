package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestNextSync(t *testing.T) {
	now := time.Date(2026, 10, 17, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		failures int
		want     time.Duration
	}{
		{0, 10 * time.Minute},
		{1, 10 * time.Minute},
		{2, 20 * time.Minute},
		{3, 40 * time.Minute},
		{20, 6 * time.Hour},
	}
	for _, tt := range tests {
		if got := NextSync(tt.failures, now).Sub(now); got != tt.want {
			t.Errorf("NextSync(%d) waits %v, want %v", tt.failures, got, tt.want)
		}
	}
}

func TestImportUnits(t *testing.T) {
	villa, house, r1, r2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	single := []Unit{{ID: villa}}
	withHouse := []Unit{{ID: house, LinkedUnitIDs: []uuid.UUID{r1, r2}}, {ID: r1}, {ID: r2}}
	roomsOnly := []Unit{{ID: r1}, {ID: r2}}
	tests := []struct {
		name       string
		units      []Unit
		feedUnit   *uuid.UUID
		wantUnit   uuid.UUID
		wantLedger int
	}{
		{"one unit", single, nil, villa, 1},
		{"a whole-house unit covers the property", withHouse, nil, house, 2},
		{"rooms only hold every room", roomsOnly, nil, r1, 2},
		{"a feed for one room", withHouse, &r2, r2, 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			unit, ledger, ok := ImportUnits(tt.units, tt.feedUnit)
			if !ok || unit != tt.wantUnit || len(ledger) != tt.wantLedger {
				t.Errorf("ImportUnits = %v, %d units, %v; want %v, %d units", unit, len(ledger), ok, tt.wantUnit, tt.wantLedger)
			}
		})
	}
}

func TestImportUnitsRefusesAnotherPropertysUnit(t *testing.T) {
	other := uuid.New()
	if _, _, ok := ImportUnits([]Unit{{ID: uuid.New()}}, &other); ok {
		t.Error("a feed unit outside the property should not import")
	}
}

func TestExportRunsCollapseNightsPerStay(t *testing.T) {
	stay, block, r1, r2 := uuid.New(), uuid.New(), uuid.New(), uuid.New()
	d := func(s string) time.Time { v, _ := time.Parse(time.DateOnly, s); return v }
	nights := []HeldNight{
		// A whole-house stay holds both rooms for two nights.
		{UnitID: r1, Night: d("2026-11-01"), BookingID: &stay},
		{UnitID: r2, Night: d("2026-11-01"), BookingID: &stay},
		{UnitID: r1, Night: d("2026-11-02"), BookingID: &stay},
		{UnitID: r2, Night: d("2026-11-02"), BookingID: &stay},
		// A block with a gap makes two runs.
		{UnitID: r2, Night: d("2026-11-10"), BlockID: &block},
		{UnitID: r2, Night: d("2026-11-12"), BlockID: &block},
	}
	runs := ExportRuns(nights, nil)
	if len(runs) != 3 || !runs[0].End.Equal(d("2026-11-03")) || runs[0].Block || !runs[2].Block {
		t.Errorf("runs = %+v", runs)
	}
	if onlyR1 := ExportRuns(nights, []uuid.UUID{r1}); len(onlyR1) != 1 {
		t.Errorf("runs for room 1 = %+v, want just the stay", onlyR1)
	}
}
