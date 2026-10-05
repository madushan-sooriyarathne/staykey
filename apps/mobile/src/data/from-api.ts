import type {
  Booking as ApiBooking,
  Calendar,
  NewProperty,
  Property,
  PropertyPatch,
} from "@staykey/api-client";
import type { Overrides } from "./pricing";
import type { Block, Booking, Currency, PropertyConfig } from "./types";

/**
 * Conversions between the API's property and the model the screens use. Screens keep reading
 * PropertyConfig; the API is the source of truth.
 */

type DisplayCurrency = NonNullable<PropertyPatch["booking"]>["displayCurrencies"][number];

/** A property from the API as the screens see it. */
export function toPropertyConfig(p: Property): PropertyConfig {
  return {
    id: p.id,
    name: p.name,
    slug: p.slug,
    bookingPageUrl: p.bookingPageUrl,
    bookingType: p.bookingType,
    location: p.location ?? "",
    description: p.description,
    amenities: p.amenities,
    checkIn: p.checkIn,
    checkOut: p.checkOut,
    photos: p.photos.map((ph) => ({ uri: ph.url, key: ph.key, caption: ph.caption || undefined })),
    currency: p.currency as Currency,
    units: p.units.map((u) => ({
      id: u.id,
      name: u.name,
      sleeps: u.sleeps,
      beds: u.beds,
      rate: u.rate,
      weekendRate: u.weekendRate ?? undefined,
      linkedUnitIds: u.linkedUnitIds.length ? u.linkedUnitIds : undefined,
    })),
    extraGuest: p.extraGuest.amount > 0 ? p.extraGuest : null,
    seasons: p.seasons.map((se) => ({
      id: se.id,
      name: se.name,
      start: se.start,
      end: se.end,
      prices: se.prices,
      minNights: se.minNights ?? undefined,
    })),
    lengthDiscounts: p.lengthDiscounts,
    rules: {
      minNights: p.rules.minNights,
      maxNights: p.rules.maxNights,
      sameDayCutoff: p.rules.sameDayCutoff ?? null,
      windowMonths: p.rules.windowMonths,
      closedArrival: p.rules.closedArrival,
    },
    policy: p.policy,
    depositPercent: p.depositPercent,
    balanceDueDays: p.balanceDueDays,
    houseRules: p.houseRules,
    charges: p.charges.map((c) => ({ ...c, note: c.note || undefined })),
    extras: p.extras,
    promos: p.promos.map((pr) => ({
      id: pr.id,
      code: pr.code,
      kind: pr.kind,
      amount: pr.amount,
      from: pr.from ?? undefined,
      to: pr.to ?? undefined,
      limit: pr.limit ?? undefined,
      used: pr.used,
      minNights: pr.minNights ?? undefined,
      note: pr.note || undefined,
    })),
    payments: p.payments,
    booking: p.booking,
    branding: {
      color: p.branding.color,
      logoUri: p.branding.logoUrl ?? undefined,
      logoKey: p.branding.logoKey ?? undefined,
    },
    ical: p.icalFeeds.map((f) => ({
      id: f.id,
      channel: f.channel,
      url: f.url,
      status: f.status,
      lastSync: f.lastSync ?? undefined,
      error: f.error ?? undefined,
      upcoming: f.upcoming,
    })),
    icalExportToken: p.icalExportToken,
  };
}

/**
 * The patch that saves some of a property's settings. Lists are sent whole; items keep their
 * ids, and new items' local ids let the server tell them apart and link them. Read-only fields
 * (address, currency, export token) are never sent.
 */
export function toPatch(changes: Partial<PropertyConfig>): PropertyPatch {
  const patch: PropertyPatch = {};
  const c = changes;
  if (c.name !== undefined) patch.name = c.name;
  if (c.bookingType !== undefined) patch.bookingType = c.bookingType;
  if (c.description !== undefined) patch.description = c.description;
  if (c.location !== undefined) patch.location = c.location;
  if (c.checkIn !== undefined) patch.checkIn = c.checkIn;
  if (c.checkOut !== undefined) patch.checkOut = c.checkOut;
  if (c.amenities !== undefined) patch.amenities = c.amenities;
  if (c.policy !== undefined) patch.policy = c.policy;
  if (c.depositPercent !== undefined) patch.depositPercent = c.depositPercent;
  if (c.balanceDueDays !== undefined) patch.balanceDueDays = c.balanceDueDays;
  if (c.houseRules !== undefined) patch.houseRules = c.houseRules;
  if (c.rules !== undefined) patch.rules = c.rules;
  if (c.booking !== undefined) {
    patch.booking = {
      ...c.booking,
      displayCurrencies: c.booking.displayCurrencies as DisplayCurrency[],
    };
  }
  if (c.branding !== undefined) {
    patch.branding = { color: c.branding.color, logoKey: c.branding.logoKey };
  }
  if (c.payments !== undefined) patch.payments = c.payments;
  if (c.extraGuest !== undefined) patch.extraGuest = c.extraGuest ?? { above: 0, amount: 0 };
  if (c.units !== undefined) {
    patch.units = c.units.map((u) => ({
      id: u.id,
      name: u.name,
      sleeps: u.sleeps,
      beds: u.beds,
      rate: u.rate,
      weekendRate: u.weekendRate,
      linkedUnitIds: u.linkedUnitIds ?? [],
    }));
  }
  if (c.seasons !== undefined) {
    patch.seasons = c.seasons.map((se) => ({
      id: se.id,
      name: se.name,
      start: se.start,
      end: se.end,
      minNights: se.minNights,
      prices: se.prices,
    }));
  }
  if (c.lengthDiscounts !== undefined) patch.lengthDiscounts = c.lengthDiscounts;
  if (c.charges !== undefined) patch.charges = c.charges;
  if (c.extras !== undefined) patch.extras = c.extras;
  if (c.promos !== undefined) {
    patch.promos = c.promos.map((pr) => ({
      id: pr.id,
      code: pr.code,
      kind: pr.kind,
      amount: pr.amount,
      from: pr.from,
      to: pr.to,
      limit: pr.limit,
      minNights: pr.minNights,
      note: pr.note,
    }));
  }
  if (c.photos !== undefined) {
    patch.photos = c.photos.flatMap((ph) => (ph.key ? [{ key: ph.key, caption: ph.caption }] : []));
  }
  if (c.ical !== undefined) {
    patch.icalFeeds = c.ical.map((f) => ({ id: f.id, channel: f.channel, url: f.url }));
  }
  return patch;
}

/** Everything a full property carries, for publishing a property built on the device. */
export function toNewProperty(p: PropertyConfig): NewProperty {
  // The basics travel at the top level, not in the setup.
  const { name: _name, bookingType: _type, location: _location, ...setup } = toPatch(p);
  return {
    name: p.name,
    slug: p.slug || undefined,
    bookingType: p.bookingType,
    location: p.location || undefined,
    currency: p.currency,
    setup,
  };
}

/**
 * A stay from the API as the screens see it. Caretakers get no money fields, which read as
 * zero. The app shows one slip per stay: the pending one, else the latest.
 */
export function toBooking(b: ApiBooking): Booking {
  const payments = (b.payments ?? []).map((p) => ({
    id: p.id,
    amount: p.amount,
    method: p.kind === "refund" ? ("refund" as const) : p.method,
    at: p.receivedAt,
    note: p.note || undefined,
  }));
  const slips = b.slips ?? [];
  const slip = slips.find((x) => x.status === "pending") ?? slips[slips.length - 1];
  return {
    id: b.id,
    ref: b.ref,
    propertyId: b.propertyId,
    unitId: b.unitId,
    source: b.source,
    status: b.status,
    guest: b.guest,
    adults: b.adults,
    children: b.children,
    checkIn: b.checkIn,
    checkOut: b.checkOut,
    lines: b.lines ?? [],
    total: b.total ?? 0,
    payments,
    slip: slip && { id: slip.id, amount: slip.amount, at: slip.uploadedAt, status: slip.status },
    extras: b.extras,
    guestNote: b.guestNote || undefined,
    ownerNote: b.ownerNote || undefined,
    createdAt: b.createdAt,
    requestExpiresAt: b.requestExpiresAt,
    cancel: b.cancel && {
      reason: b.cancel.reason,
      at: b.cancel.at,
      refund: payments.reduce((n, p) => n + (p.method === "refund" ? p.amount : 0), 0),
    },
    version: b.version,
  };
}

/** A property's blocks and rate overrides as the calendar and pricing read them. */
export function toCalendar(c: Calendar): { blocks: Block[]; overrides: Overrides } {
  const overrides: Overrides = {};
  for (const o of c.rateOverrides) {
    const unit = overrides[o.unitId] ?? {};
    unit[o.night] = {
      price: o.price,
      minNights: o.minNights,
      closedToArrival: o.closedToArrival || undefined,
    };
    overrides[o.unitId] = unit;
  }
  return { blocks: c.blocks.map((b) => ({ ...b, note: b.note || undefined })), overrides };
}
