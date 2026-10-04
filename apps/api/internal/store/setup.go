package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"

	"staykey.direct/api/internal/domain"
	"staykey.direct/api/internal/store/queries"
)

// loadSetup fills in the collections of each property: units, photos, pricing, payment settings
// and calendar feeds, one query per collection for the whole page.
func (s *Postgres) loadSetup(ctx context.Context, accountID uuid.UUID, details []domain.PropertyDetail) error {
	if len(details) == 0 {
		return nil
	}
	ids := make([]uuid.UUID, len(details))
	byID := make(map[uuid.UUID]*domain.PropertyDetail, len(details))
	for i := range details {
		ids[i] = details[i].ID
		byID[details[i].ID] = &details[i]
	}
	q := s.db(ctx)

	units, err := q.ListUnits(ctx, queries.ListUnitsParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	links, err := q.ListUnitLinks(ctx, queries.ListUnitLinksParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	children := map[uuid.UUID][]uuid.UUID{}
	for _, l := range links {
		children[l.ParentUnitID] = append(children[l.ParentUnitID], l.ChildUnitID)
	}
	for _, u := range units {
		unit := domain.Unit{
			ID: u.ID, Name: u.Name, Sleeps: int(u.Sleeps), Beds: u.Beds, Rate: u.Rate,
			LinkedUnitIDs: orEmpty(children[u.ID]),
		}
		if u.WeekendRate != nil {
			unit.WeekendRate = *u.WeekendRate
		}
		byID[u.PropertyID].Units = append(byID[u.PropertyID].Units, unit)
	}

	photos, err := q.ListPhotos(ctx, queries.ListPhotosParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, p := range photos {
		byID[p.PropertyID].Photos = append(byID[p.PropertyID].Photos, domain.Photo{ID: p.ID, Key: p.Key, Caption: p.Caption})
	}

	payments, err := q.ListPaymentSettings(ctx, queries.ListPaymentSettingsParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, p := range payments {
		number, err := s.openAccountNumber(p.PropertyID, p.AccountNumberEnc)
		if err != nil {
			return err
		}
		byID[p.PropertyID].Payments = domain.PaymentSettings{
			Bank: domain.BankDetails{
				Enabled: p.BankEnabled, BankName: p.BankName, AccountName: p.AccountName,
				AccountNumber: number, PayWithinHours: int(p.PayWithinHours), CancelIfUnpaid: p.CancelIfUnpaid,
			},
			AtProperty: p.AtProperty,
			Cards:      p.CardsStatus,
		}
	}

	seasons, err := q.ListSeasons(ctx, queries.ListSeasonsParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	prices, err := q.ListSeasonPrices(ctx, queries.ListSeasonPricesParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	pricesBySeason := map[uuid.UUID]map[uuid.UUID]int64{}
	for _, p := range prices {
		if pricesBySeason[p.SeasonID] == nil {
			pricesBySeason[p.SeasonID] = map[uuid.UUID]int64{}
		}
		pricesBySeason[p.SeasonID][p.UnitID] = p.Price
	}
	for _, se := range seasons {
		season := domain.Season{ID: se.ID, Name: se.Name, Start: se.StartMd, End: se.EndMd, Prices: pricesBySeason[se.ID]}
		if season.Prices == nil {
			season.Prices = map[uuid.UUID]int64{}
		}
		if se.MinNights != nil {
			season.MinNights = int(*se.MinNights)
		}
		byID[se.PropertyID].Seasons = append(byID[se.PropertyID].Seasons, season)
	}

	discounts, err := q.ListLengthDiscounts(ctx, queries.ListLengthDiscountsParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, d := range discounts {
		byID[d.PropertyID].LengthDiscounts = append(byID[d.PropertyID].LengthDiscounts,
			domain.LengthDiscount{Nights: int(d.Nights), Percent: int(d.Percent)})
	}

	charges, err := q.ListCharges(ctx, queries.ListChargesParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, c := range charges {
		byID[c.PropertyID].Charges = append(byID[c.PropertyID].Charges, domain.Charge{
			ID: c.ID, Name: c.Name, Kind: c.Kind, Amount: c.Amount, Per: c.Per, Enabled: c.Enabled, Note: c.Note,
		})
	}

	extras, err := q.ListExtras(ctx, queries.ListExtrasParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, e := range extras {
		byID[e.PropertyID].Extras = append(byID[e.PropertyID].Extras, domain.Extra{
			ID: e.ID, Name: e.Name, Price: e.Price, Per: e.Per, OnRequest: e.OnRequest, Enabled: e.Enabled,
		})
	}

	promos, err := q.ListPromos(ctx, queries.ListPromosParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, p := range promos {
		promo := domain.Promo{
			ID: p.ID, Code: p.Code, Kind: p.Kind, Amount: p.Amount, Used: int(p.UsedCount), Note: p.Note,
			From: day(p.StartsOn), To: day(p.EndsOn),
		}
		if p.UsageLimit != nil {
			promo.Limit = int(*p.UsageLimit)
		}
		if p.MinNights != nil {
			promo.MinNights = int(*p.MinNights)
		}
		byID[p.PropertyID].Promos = append(byID[p.PropertyID].Promos, promo)
	}

	feeds, err := q.ListIcalFeeds(ctx, queries.ListIcalFeedsParams{AccountID: accountID, PropertyIds: ids})
	if err != nil {
		return err
	}
	for _, f := range feeds {
		feed := domain.IcalFeed{
			ID: f.ID, Channel: f.Channel, URL: f.Url, Status: f.Status, LastSync: f.LastSyncedAt, Upcoming: int(f.Upcoming),
		}
		if f.LastError != nil {
			feed.Error = *f.LastError
		}
		byID[f.PropertyID].IcalFeeds = append(byID[f.PropertyID].IcalFeeds, feed)
	}
	return nil
}

// applySetup writes every section the patch carries. It runs inside the caller's transaction,
// after the property row is locked or created.
func (s *Postgres) applySetup(ctx context.Context, accountID, propertyID uuid.UUID, p domain.PropertyPatch) error {
	q := s.db(ctx)

	if p.Name != nil || p.BookingType != nil || p.Description != nil || p.Location != nil ||
		p.CheckIn != nil || p.CheckOut != nil || p.Amenities != nil || p.Policy != nil ||
		p.DepositPercent != nil || p.BalanceDueDays != nil || p.HouseRules != nil {
		params := queries.UpdatePropertyDetailsParams{
			ID: propertyID, AccountID: accountID,
			Name: p.Name, Description: p.Description, Location: p.Location,
			CheckInTime: p.CheckIn, CheckOutTime: p.CheckOut, Policy: p.Policy,
			DepositPercent: small(p.DepositPercent), BalanceDueDays: small(p.BalanceDueDays),
		}
		if p.BookingType != nil {
			bt := string(*p.BookingType)
			params.BookingType = &bt
		}
		if p.Amenities != nil {
			params.Amenities = orEmpty(*p.Amenities)
		}
		if p.HouseRules != nil {
			params.HouseRules = orEmpty(*p.HouseRules)
		}
		if err := q.UpdatePropertyDetails(ctx, params); err != nil {
			return err
		}
	}

	if r := p.Rules; r != nil {
		closed := make([]int16, len(r.ClosedArrival))
		for i, d := range r.ClosedArrival {
			closed[i] = int16(d)
		}
		if err := q.UpdatePropertyRules(ctx, queries.UpdatePropertyRulesParams{
			ID: propertyID, AccountID: accountID,
			MinNights: int16(r.MinNights), MaxNights: int16(r.MaxNights), SameDayCutoff: small(r.SameDayCutoff),
			WindowMonths: int16(r.WindowMonths), ClosedArrival: closed,
		}); err != nil {
			return err
		}
	}

	if b := p.Booking; b != nil {
		if err := q.UpdatePropertyBooking(ctx, queries.UpdatePropertyBookingParams{
			ID: propertyID, AccountID: accountID, BookingMode: b.Mode, ReplyHours: int16(b.ReplyHours),
			HoldMinutes: int16(b.HoldMinutes), DisplayCurrencies: b.DisplayCurrencies,
		}); err != nil {
			return err
		}
	}

	if b := p.Branding; b != nil {
		if err := q.UpdatePropertyBranding(ctx, queries.UpdatePropertyBrandingParams{
			ID: propertyID, AccountID: accountID, BrandColor: b.Color, LogoKey: nonEmpty(b.LogoKey),
		}); err != nil {
			return err
		}
	}

	if g := p.ExtraGuest; g != nil {
		above := g.Above
		if g.Amount == 0 {
			above = 0
		}
		if err := q.UpdatePropertyExtraGuest(ctx, queries.UpdatePropertyExtraGuestParams{
			ID: propertyID, AccountID: accountID, ExtraGuestAbove: int16(above), ExtraGuestAmount: g.Amount,
		}); err != nil {
			return err
		}
	}

	if p.Payments != nil {
		if err := s.applyPayments(ctx, accountID, propertyID, *p.Payments); err != nil {
			return err
		}
	}

	// Unit refs resolve to ids for links and season prices.
	unitIDs, err := q.UnitIDs(ctx, queries.UnitIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	unitRef := map[string]uuid.UUID{}
	for _, id := range unitIDs {
		unitRef[id.String()] = id
	}
	if p.Units != nil {
		if unitRef, err = s.applyUnits(ctx, accountID, propertyID, *p.Units, set(unitIDs)); err != nil {
			return err
		}
	}

	if p.Seasons != nil {
		if err := s.applySeasons(ctx, accountID, propertyID, *p.Seasons, unitRef); err != nil {
			return err
		}
	}

	if p.LengthDiscounts != nil {
		if err := q.DeleteLengthDiscounts(ctx, queries.DeleteLengthDiscountsParams{PropertyID: propertyID, AccountID: accountID}); err != nil {
			return err
		}
		for _, d := range *p.LengthDiscounts {
			if err := q.InsertLengthDiscount(ctx, queries.InsertLengthDiscountParams{
				AccountID: accountID, PropertyID: propertyID, Nights: int16(d.Nights), Percent: int16(d.Percent),
			}); err != nil {
				return err
			}
		}
	}

	if p.Charges != nil {
		if err := s.applyCharges(ctx, accountID, propertyID, *p.Charges); err != nil {
			return err
		}
	}
	if p.Extras != nil {
		if err := s.applyExtras(ctx, accountID, propertyID, *p.Extras); err != nil {
			return err
		}
	}
	if p.Promos != nil {
		if err := s.applyPromos(ctx, accountID, propertyID, *p.Promos); err != nil {
			return err
		}
	}
	if p.IcalFeeds != nil {
		if err := s.applyFeeds(ctx, accountID, propertyID, *p.IcalFeeds); err != nil {
			return err
		}
	}

	if p.Photos != nil {
		if err := q.DeletePhotos(ctx, queries.DeletePhotosParams{PropertyID: propertyID, AccountID: accountID}); err != nil {
			return err
		}
		for i, ph := range *p.Photos {
			if err := q.InsertPhoto(ctx, queries.InsertPhotoParams{
				ID: domain.NewID(), AccountID: accountID, PropertyID: propertyID, Key: ph.Key,
				Caption: ph.Caption, Position: int16(i),
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Postgres) applyPayments(ctx context.Context, accountID, propertyID uuid.UUID, pay domain.PaymentSettings) error {
	q := s.db(ctx)
	current, err := q.ListPaymentSettings(ctx, queries.ListPaymentSettingsParams{AccountID: accountID, PropertyIds: []uuid.UUID{propertyID}})
	if err != nil {
		return err
	}
	// Card payments turn on when PayHere approves them, never from the app.
	if pay.Cards == "on" && (len(current) == 0 || current[0].CardsStatus != "on") {
		return &domain.ValidationError{Field: "payments.cards", Message: "card payments turn on once PayHere approves them"}
	}

	var enc []byte
	if pay.Bank.AccountNumber != "" {
		if enc, err = s.sealAccountNumber(propertyID, pay.Bank.AccountNumber); err != nil {
			return err
		}
	}
	return q.UpsertPaymentSettings(ctx, queries.UpsertPaymentSettingsParams{
		PropertyID: propertyID, AccountID: accountID, BankEnabled: pay.Bank.Enabled,
		BankName: pay.Bank.BankName, AccountName: pay.Bank.AccountName, AccountNumberEnc: enc,
		PayWithinHours: int16(pay.Bank.PayWithinHours), CancelIfUnpaid: pay.Bank.CancelIfUnpaid,
		AtProperty: pay.AtProperty, CardsStatus: pay.Cards,
	})
}

func (s *Postgres) applyUnits(ctx context.Context, accountID, propertyID uuid.UUID, units []domain.UnitInput, existing map[uuid.UUID]bool) (map[string]uuid.UUID, error) {
	q := s.db(ctx)
	ref := map[string]uuid.UUID{}
	keep := make([]uuid.UUID, 0, len(units))
	for i, u := range units {
		var weekend *int64
		if u.WeekendRate > 0 {
			weekend = &u.WeekendRate
		}
		if id, ok := domain.RefID(u.Ref, existing); ok {
			if err := q.UpdateUnit(ctx, queries.UpdateUnitParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Name: u.Name, Sleeps: int16(u.Sleeps),
				Beds: u.Beds, Rate: u.Rate, WeekendRate: weekend, Position: int16(i),
			}); err != nil {
				return nil, err
			}
			ref[u.Ref] = id
			keep = append(keep, id)
			continue
		}
		id := domain.NewID()
		if err := q.InsertUnit(ctx, queries.InsertUnitParams{
			ID: id, AccountID: accountID, PropertyID: propertyID, Name: u.Name, Sleeps: int16(u.Sleeps),
			Beds: u.Beds, Rate: u.Rate, WeekendRate: weekend, Position: int16(i),
		}); err != nil {
			return nil, err
		}
		ref[u.Ref] = id
		keep = append(keep, id)
	}

	archived, err := q.ArchiveUnitsNotIn(ctx, queries.ArchiveUnitsNotInParams{
		PropertyID: propertyID, AccountID: accountID, Keep: keep, Now: time.Now(),
	})
	if err != nil {
		return nil, err
	}
	if len(archived) > 0 {
		if err := q.DeleteSeasonPricesForUnits(ctx, queries.DeleteSeasonPricesForUnitsParams{
			PropertyID: propertyID, AccountID: accountID, UnitIds: archived,
		}); err != nil {
			return nil, err
		}
	}

	if err := q.DeleteUnitLinks(ctx, queries.DeleteUnitLinksParams{PropertyID: propertyID, AccountID: accountID}); err != nil {
		return nil, err
	}
	for _, u := range units {
		for _, child := range u.LinkedUnitRef {
			if err := q.InsertUnitLink(ctx, queries.InsertUnitLinkParams{
				AccountID: accountID, PropertyID: propertyID, ParentUnitID: ref[u.Ref], ChildUnitID: ref[child],
			}); err != nil {
				return nil, err
			}
		}
	}
	return ref, nil
}

func (s *Postgres) applySeasons(ctx context.Context, accountID, propertyID uuid.UUID, seasons []domain.SeasonInput, unitRef map[string]uuid.UUID) error {
	q := s.db(ctx)
	ids, err := q.SeasonIDs(ctx, queries.SeasonIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	existing := set(ids)
	keep := make([]uuid.UUID, 0, len(seasons))
	seasonIDs := make([]uuid.UUID, len(seasons))
	for i, se := range seasons {
		minNights := small(nonZero(se.MinNights))
		if id, ok := domain.RefID(se.Ref, existing); ok {
			if err := q.UpdateSeason(ctx, queries.UpdateSeasonParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Name: se.Name, StartMd: se.Start,
				EndMd: se.End, MinNights: minNights, Position: int16(i),
			}); err != nil {
				return err
			}
			seasonIDs[i] = id
		} else {
			seasonIDs[i] = domain.NewID()
			if err := q.InsertSeason(ctx, queries.InsertSeasonParams{
				ID: seasonIDs[i], AccountID: accountID, PropertyID: propertyID, Name: se.Name,
				StartMd: se.Start, EndMd: se.End, MinNights: minNights, Position: int16(i),
			}); err != nil {
				return err
			}
		}
		keep = append(keep, seasonIDs[i])
	}
	if err := q.DeleteSeasonsNotIn(ctx, queries.DeleteSeasonsNotInParams{PropertyID: propertyID, AccountID: accountID, Keep: keep}); err != nil {
		return err
	}
	if err := q.DeleteSeasonPrices(ctx, queries.DeleteSeasonPricesParams{PropertyID: propertyID, AccountID: accountID}); err != nil {
		return err
	}
	for i, se := range seasons {
		for ref, price := range se.Prices {
			unitID, ok := unitRef[ref]
			if !ok {
				return &domain.ValidationError{Field: fmt.Sprintf("seasons[%d].prices", i), Message: "must be for units of this property"}
			}
			if err := q.InsertSeasonPrice(ctx, queries.InsertSeasonPriceParams{
				AccountID: accountID, PropertyID: propertyID, SeasonID: seasonIDs[i], UnitID: unitID, Price: price,
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Postgres) applyCharges(ctx context.Context, accountID, propertyID uuid.UUID, charges []domain.ChargeInput) error {
	q := s.db(ctx)
	ids, err := q.ChargeIDs(ctx, queries.ChargeIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	existing := set(ids)
	keep := make([]uuid.UUID, 0, len(charges))
	for i, c := range charges {
		if id, ok := domain.RefID(c.Ref, existing); ok {
			err = q.UpdateCharge(ctx, queries.UpdateChargeParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Name: c.Name, Kind: c.Kind,
				Amount: c.Amount, Per: c.Per, Enabled: c.Enabled, Note: c.Note, Position: int16(i),
			})
			keep = append(keep, id)
		} else {
			id := domain.NewID()
			err = q.InsertCharge(ctx, queries.InsertChargeParams{
				ID: id, AccountID: accountID, PropertyID: propertyID, Name: c.Name, Kind: c.Kind,
				Amount: c.Amount, Per: c.Per, Enabled: c.Enabled, Note: c.Note, Position: int16(i),
			})
			keep = append(keep, id)
		}
		if err != nil {
			return err
		}
	}
	return q.DeleteChargesNotIn(ctx, queries.DeleteChargesNotInParams{PropertyID: propertyID, AccountID: accountID, Keep: keep})
}

func (s *Postgres) applyExtras(ctx context.Context, accountID, propertyID uuid.UUID, extras []domain.ExtraInput) error {
	q := s.db(ctx)
	ids, err := q.ExtraIDs(ctx, queries.ExtraIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	existing := set(ids)
	keep := make([]uuid.UUID, 0, len(extras))
	for i, e := range extras {
		if id, ok := domain.RefID(e.Ref, existing); ok {
			err = q.UpdateExtra(ctx, queries.UpdateExtraParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Name: e.Name, Price: e.Price,
				Per: e.Per, OnRequest: e.OnRequest, Enabled: e.Enabled, Position: int16(i),
			})
			keep = append(keep, id)
		} else {
			id := domain.NewID()
			err = q.InsertExtra(ctx, queries.InsertExtraParams{
				ID: id, AccountID: accountID, PropertyID: propertyID, Name: e.Name, Price: e.Price,
				Per: e.Per, OnRequest: e.OnRequest, Enabled: e.Enabled, Position: int16(i),
			})
			keep = append(keep, id)
		}
		if err != nil {
			return err
		}
	}
	return q.DeleteExtrasNotIn(ctx, queries.DeleteExtrasNotInParams{PropertyID: propertyID, AccountID: accountID, Keep: keep})
}

func (s *Postgres) applyPromos(ctx context.Context, accountID, propertyID uuid.UUID, promos []domain.PromoInput) error {
	q := s.db(ctx)
	ids, err := q.PromoIDs(ctx, queries.PromoIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	existing := set(ids)
	keep := []uuid.UUID{}
	// Remove dropped codes first, so a code can move from a deleted promo to a new one.
	for _, p := range promos {
		if id, ok := domain.RefID(p.Ref, existing); ok {
			keep = append(keep, id)
		}
	}
	if err := q.DeletePromosNotIn(ctx, queries.DeletePromosNotInParams{PropertyID: propertyID, AccountID: accountID, Keep: keep}); err != nil {
		return err
	}
	for i, p := range promos {
		from, to := date(p.From), date(p.To)
		limit := int32ptr(p.Limit)
		minNights := small(nonZero(p.MinNights))
		if id, ok := domain.RefID(p.Ref, existing); ok {
			err = q.UpdatePromo(ctx, queries.UpdatePromoParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Code: p.Code, Kind: p.Kind,
				Amount: p.Amount, StartsOn: from, EndsOn: to, UsageLimit: limit, MinNights: minNights,
				Note: p.Note, Position: int16(i),
			})
		} else {
			err = q.InsertPromo(ctx, queries.InsertPromoParams{
				ID: domain.NewID(), AccountID: accountID, PropertyID: propertyID, Code: p.Code,
				Kind: p.Kind, Amount: p.Amount, StartsOn: from, EndsOn: to, UsageLimit: limit,
				MinNights: minNights, Note: p.Note, Position: int16(i),
			})
		}
		if violates(err, "promos_property_id_code_key") {
			return &domain.ValidationError{Field: fmt.Sprintf("promos[%d].code", i), Message: "is already used by another code"}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Postgres) applyFeeds(ctx context.Context, accountID, propertyID uuid.UUID, feeds []domain.IcalFeedInput) error {
	q := s.db(ctx)
	ids, err := q.IcalFeedIDs(ctx, queries.IcalFeedIDsParams{PropertyID: propertyID, AccountID: accountID})
	if err != nil {
		return err
	}
	existing := set(ids)
	keep := make([]uuid.UUID, 0, len(feeds))
	for i, f := range feeds {
		if id, ok := domain.RefID(f.Ref, existing); ok {
			err = q.UpdateIcalFeed(ctx, queries.UpdateIcalFeedParams{
				ID: id, PropertyID: propertyID, AccountID: accountID, Channel: f.Channel, Url: f.URL, Position: int16(i),
			})
			keep = append(keep, id)
		} else {
			id := domain.NewID()
			err = q.InsertIcalFeed(ctx, queries.InsertIcalFeedParams{
				ID: id, AccountID: accountID, PropertyID: propertyID, Channel: f.Channel, Url: f.URL, Position: int16(i),
			})
			keep = append(keep, id)
		}
		if err != nil {
			return err
		}
	}
	return q.DeleteIcalFeedsNotIn(ctx, queries.DeleteIcalFeedsNotInParams{PropertyID: propertyID, AccountID: accountID, Keep: keep})
}

var errNoSecrets = errors.New("store has no secret box; pass store.WithSecrets")

func (s *Postgres) sealAccountNumber(propertyID uuid.UUID, number string) ([]byte, error) {
	if s.secrets == nil {
		return nil, errNoSecrets
	}
	return s.secrets.Seal([]byte(number), propertyID[:])
}

func (s *Postgres) openAccountNumber(propertyID uuid.UUID, sealed []byte) (string, error) {
	if len(sealed) == 0 {
		return "", nil
	}
	if s.secrets == nil {
		return "", errNoSecrets
	}
	plain, err := s.secrets.Open(sealed, propertyID[:])
	if err != nil {
		return "", fmt.Errorf("bank account for property %s: %w", propertyID, err)
	}
	return string(plain), nil
}

func set(ids []uuid.UUID) map[uuid.UUID]bool {
	m := make(map[uuid.UUID]bool, len(ids))
	for _, id := range ids {
		m[id] = true
	}
	return m
}

func orEmpty[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func small(v *int) *int16 {
	if v == nil {
		return nil
	}
	n := int16(*v)
	return &n
}

func nonZero(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

func int32ptr(v int) *int32 {
	if v == 0 {
		return nil
	}
	n := int32(v)
	return &n
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func date(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.DateOnly, s)
	if err != nil {
		return nil
	}
	return &t
}

func day(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.DateOnly)
}
