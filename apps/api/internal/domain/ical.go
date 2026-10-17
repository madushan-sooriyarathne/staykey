package domain

import (
	"slices"
	"sort"
	"time"

	"github.com/google/uuid"
)

// SyncInterval is how often a working feed is read. With the import job's one minute tick, an OTA
// stay closes the booking page within 15 minutes of reaching the OTA's export.
const SyncInterval = 10 * time.Minute

// FeedErrorAfter and FeedAlertAfter are the failures in a row before a feed shows as failing, and
// before the team is told.
const (
	FeedErrorAfter = 2
	FeedAlertAfter = 3
)

// ChannelNames are the OTA names people read, by feed channel.
var ChannelNames = map[string]string{
	"airbnb": "Airbnb", "booking": "Booking.com", "agoda": "Agoda", "expedia": "Expedia", "other": "Calendar",
}

// ChannelName is a feed channel's name for people.
func ChannelName(channel string) string {
	if n, ok := ChannelNames[channel]; ok {
		return n
	}
	return "Calendar"
}

// NextSync is when a feed is read again after failures failures in a row: SyncInterval when it
// works, then 10, 20, 40 minutes and so on, capped at six hours.
func NextSync(failures int, now time.Time) time.Time {
	if failures <= 0 {
		return now.Add(SyncInterval)
	}
	wait := 10 * time.Minute
	for range failures - 1 {
		wait *= 2
		if wait >= 6*time.Hour {
			return now.Add(6 * time.Hour)
		}
	}
	return now.Add(wait)
}

// ImportUnits decides which unit an imported stay is recorded on and which units' nights it
// holds. A feed tied to a unit uses that unit. Otherwise it covers the whole property: its only
// unit, else its whole-house unit, else every unit, recorded on the first. units are the
// property's active units in order.
func ImportUnits(units []Unit, feedUnitID *uuid.UUID) (unit uuid.UUID, ledger []uuid.UUID, ok bool) {
	if len(units) == 0 {
		return uuid.Nil, nil, false
	}
	if feedUnitID != nil {
		if !slices.ContainsFunc(units, func(u Unit) bool { return u.ID == *feedUnitID }) {
			return uuid.Nil, nil, false
		}
		return *feedUnitID, LedgerUnits(units, *feedUnitID), true
	}
	if len(units) == 1 {
		return units[0].ID, LedgerUnits(units, units[0].ID), true
	}
	for _, u := range units {
		if len(u.LinkedUnitIDs) > 0 {
			return u.ID, LedgerUnits(units, u.ID), true
		}
	}
	all := make([]uuid.UUID, len(units))
	for i, u := range units {
		all[i] = u.ID
	}
	return units[0].ID, all, true
}

// HeldNight is one ledger row the export reads.
type HeldNight struct {
	UnitID    uuid.UUID
	Night     time.Time
	BookingID *uuid.UUID
	BlockID   *uuid.UUID
}

// ExportRun is a run of nights one stay or block holds, [Start, End).
type ExportRun struct {
	Start, End time.Time
	Block      bool
	// ID is the stay's or block's id, which makes the event's UID stable between exports.
	ID uuid.UUID
}

// ExportRuns collapses held nights into one run per stay or block per stretch of nights. Only
// nights on the given units count; nil means every unit.
func ExportRuns(nights []HeldNight, units []uuid.UUID) []ExportRun {
	type key struct {
		id    uuid.UUID
		block bool
	}
	days := map[key]map[time.Time]bool{}
	for _, n := range nights {
		if units != nil && !slices.Contains(units, n.UnitID) {
			continue
		}
		var k key
		switch {
		case n.BookingID != nil:
			k = key{*n.BookingID, false}
		case n.BlockID != nil:
			k = key{*n.BlockID, true}
		default:
			continue
		}
		if days[k] == nil {
			days[k] = map[time.Time]bool{}
		}
		days[k][n.Night] = true
	}
	var runs []ExportRun
	for k, set := range days {
		sorted := make([]time.Time, 0, len(set))
		for d := range set {
			sorted = append(sorted, d)
		}
		sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
		start := sorted[0]
		for i := 1; i <= len(sorted); i++ {
			if i < len(sorted) && sorted[i].Equal(sorted[i-1].AddDate(0, 0, 1)) {
				continue
			}
			runs = append(runs, ExportRun{Start: start, End: sorted[i-1].AddDate(0, 0, 1), Block: k.block, ID: k.id})
			if i < len(sorted) {
				start = sorted[i]
			}
		}
	}
	sort.Slice(runs, func(i, j int) bool {
		if !runs[i].Start.Equal(runs[j].Start) {
			return runs[i].Start.Before(runs[j].Start)
		}
		return runs[i].ID.String() < runs[j].ID.String()
	})
	return runs
}
