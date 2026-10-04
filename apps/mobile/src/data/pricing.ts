import { addDays, eachNight, inSeason, isWeekendNight, nightsBetween, today } from "./dates";
import type {
  Block,
  Booking,
  BookingStatus,
  Extra,
  ISODate,
  Policy,
  PriceLine,
  PropertyConfig,
  RateOverride,
  Source,
  Unit,
} from "./types";

export type Overrides = Record<string, Record<ISODate, RateOverride>>;

/** Statuses that hold the dates on the calendar. */
export const HOLDING: BookingStatus[] = [
  "requested",
  "awaiting_payment",
  "confirmed",
  "checked_in",
  "checked_out",
];

export const OTA_SOURCES: Source[] = ["airbnb", "booking", "agoda", "expedia"];

export const SOURCE_LABEL: Record<Source, string> = {
  page: "Booking page",
  widget: "Website widget",
  whatsapp: "WhatsApp",
  phone: "Phone",
  walkin: "Walk-in",
  other: "Other",
  airbnb: "Airbnb",
  booking: "Booking.com",
  agoda: "Agoda",
  expedia: "Expedia",
};

export const isOTA = (b: Pick<Booking, "source">) => OTA_SOURCES.includes(b.source);
export const isDirect = (b: Pick<Booking, "source">) => !isOTA(b);

export function unitOf(p: PropertyConfig, unitId: string): Unit | undefined {
  return p.units.find((u) => u.id === unitId);
}

/** The price for one night of one unit: a manual override, then a season, then the base rate. */
export function nightlyRate(
  p: PropertyConfig,
  unitId: string,
  night: ISODate,
  overrides?: Overrides,
): number {
  const o = overrides?.[unitId]?.[night]?.price;
  if (o != null) return o;
  const season = p.seasons.find((s) => inSeason(night, s.start, s.end) && s.prices[unitId] != null);
  if (season) return season.prices[unitId] as number;
  const unit = unitOf(p, unitId);
  if (!unit) return 0;
  return isWeekendNight(night) ? (unit.weekendRate ?? unit.rate) : unit.rate;
}

export function minNightsFor(
  p: PropertyConfig,
  unitId: string,
  checkIn: ISODate,
  overrides?: Overrides,
): number {
  const o = overrides?.[unitId]?.[checkIn]?.minNights;
  if (o != null) return o;
  const season = p.seasons.find((s) => inSeason(checkIn, s.start, s.end) && s.minNights);
  return season?.minNights ?? p.rules.minNights;
}

function extraTotal(e: Extra, nights: number, guests: number): number {
  switch (e.per) {
    case "stay":
    case "trip":
      return e.price;
    case "night":
      return e.price * nights;
    case "guest":
      return e.price * guests;
    case "guestNight":
      return e.price * guests * nights;
  }
}

export type Quote = {
  nights: number;
  nightly: { night: ISODate; price: number }[];
  lines: PriceLine[];
  total: number;
};

/** The guest's price breakdown, line by line, as the booking page shows it at checkout. */
export function quote(
  p: PropertyConfig,
  input: {
    unitId: string;
    from: ISODate;
    to: ISODate;
    adults: number;
    children?: number;
    extras?: string[];
    promo?: string;
    overrides?: Overrides;
  },
): Quote {
  const nights = Math.max(0, nightsBetween(input.from, input.to));
  const nightly = eachNight(input.from, input.to).map((night) => ({
    night,
    price: nightlyRate(p, input.unitId, night, input.overrides),
  }));
  const guests = input.adults + (input.children ?? 0);
  const lines: PriceLine[] = [];
  if (nights === 0) return { nights, nightly, lines, total: 0 };

  const roomTotal = nightly.reduce((n, x) => n + x.price, 0);
  lines.push({ label: `${nights} ${nights === 1 ? "night" : "nights"}`, amount: roomTotal });

  let room = roomTotal;
  const los = [...p.lengthDiscounts]
    .sort((a, b) => b.nights - a.nights)
    .find((d) => nights >= d.nights);
  if (los) {
    const off = Math.round((roomTotal * los.percent) / 100);
    lines.push({
      label: `${los.nights}+ nights, ${los.percent}% off`,
      amount: -off,
      kind: "discount",
    });
    room -= off;
  }

  const promo = input.promo ? p.promos.find((x) => x.code === input.promo) : undefined;
  if (promo) {
    const off =
      promo.kind === "percent"
        ? Math.round((room * promo.amount) / 100)
        : Math.min(room, promo.amount);
    lines.push({ label: `Code ${promo.code}`, amount: -off, kind: "discount" });
    room -= off;
  }

  if (p.extraGuest && guests > p.extraGuest.above) {
    const extra = (guests - p.extraGuest.above) * p.extraGuest.amount * nights;
    lines.push({
      label: `${guests - p.extraGuest.above} extra ${guests - p.extraGuest.above === 1 ? "guest" : "guests"}`,
      amount: extra,
    });
    room += extra;
  }

  for (const id of input.extras ?? []) {
    const e = p.extras.find((x) => x.id === id);
    if (e) lines.push({ label: e.name, amount: extraTotal(e, nights, guests), kind: "extra" });
  }

  for (const c of p.charges.filter((x) => x.enabled)) {
    const amount =
      c.kind === "percent"
        ? Math.round((room * c.amount) / 100)
        : c.per === "night"
          ? c.amount * nights
          : c.per === "guest"
            ? c.amount * guests
            : c.amount;
    lines.push({
      label: c.kind === "percent" ? `${c.name} ${c.amount}%` : c.name,
      amount,
      kind: "charge",
    });
  }

  const total = lines.reduce((n, l) => n + l.amount, 0);
  return { nights, nightly, lines, total };
}

export function paidOf(b: Booking): number {
  return b.payments.reduce((n, x) => n + (x.method === "refund" ? -x.amount : x.amount), 0);
}

export function balanceOf(b: Booking): number {
  if (isOTA(b) || b.status === "cancelled" || b.status === "declined") return 0;
  return Math.max(0, b.total - paidOf(b));
}

export function depositFor(p: PropertyConfig, total: number): number {
  return Math.round((total * p.depositPercent) / 100);
}

export const POLICY_TEXT: Record<Policy, { label: string; summary: string }> = {
  flexible: { label: "Flexible", summary: "Full refund up to 7 days before arrival" },
  moderate: { label: "Moderate", summary: "Full refund up to 14 days before arrival, then 50%" },
  strict: { label: "Strict", summary: "50% refund up to 30 days before arrival, then none" },
};

/** What the policy says to refund if the booking were cancelled today. */
export function suggestedRefund(
  policy: Policy,
  b: Booking,
  on = today(),
): { amount: number; share: number; daysOut: number } {
  const paid = paidOf(b);
  const daysOut = nightsBetween(on, b.checkIn);
  let share = 0;
  if (policy === "flexible") share = daysOut >= 7 ? 1 : 0;
  if (policy === "moderate") share = daysOut >= 14 ? 1 : 0.5;
  if (policy === "strict") share = daysOut >= 30 ? 0.5 : 0;
  if (daysOut < 0) share = 0;
  return { amount: Math.round(paid * share), share, daysOut };
}

/** Units that share inventory with this one: its linked rooms, or any whole-house unit that links it. */
export function relatedUnits(p: PropertyConfig, unitId: string): string[] {
  const unit = unitOf(p, unitId);
  const ids = new Set<string>([unitId, ...(unit?.linkedUnitIds ?? [])]);
  for (const u of p.units) if (u.linkedUnitIds?.includes(unitId)) ids.add(u.id);
  return [...ids];
}

const overlaps = (aFrom: ISODate, aTo: ISODate, bFrom: ISODate, bTo: ISODate) =>
  aFrom < bTo && bFrom < aTo;

/** Bookings and blocks that would clash with a stay on this unit. */
export function conflicts(
  p: PropertyConfig,
  bookings: Booking[],
  blocks: Block[],
  input: { unitId: string; from: ISODate; to: ISODate; ignoreBookingId?: string },
): { bookings: Booking[]; blocks: Block[] } {
  const ids = relatedUnits(p, input.unitId);
  return {
    bookings: bookings.filter(
      (b) =>
        b.propertyId === p.id &&
        b.id !== input.ignoreBookingId &&
        HOLDING.includes(b.status) &&
        ids.includes(b.unitId) &&
        overlaps(b.checkIn, b.checkOut, input.from, input.to),
    ),
    blocks: blocks.filter(
      (k) =>
        k.propertyId === p.id &&
        k.unitIds.some((u) => ids.includes(u)) &&
        overlaps(k.from, k.to, input.from, input.to),
    ),
  };
}

/** Units guests actually sleep in. A linked whole-house unit doesn't add capacity. */
export function physicalUnits(p: PropertyConfig): Unit[] {
  const linked = p.units.filter((u) => u.linkedUnitIds?.length);
  return linked.length && p.units.length > linked.length
    ? p.units.filter((u) => !u.linkedUnitIds?.length)
    : p.units;
}

/** Share of available unit nights booked between two dates. */
export function occupancy(
  p: PropertyConfig,
  bookings: Booking[],
  from: ISODate,
  to: ISODate,
): number {
  const units = physicalUnits(p);
  const nights = nightsBetween(from, to);
  if (!units.length || nights <= 0) return 0;
  let booked = 0;
  for (const b of bookings) {
    if (b.propertyId !== p.id || !HOLDING.includes(b.status) || b.status === "requested") continue;
    const linked = unitOf(p, b.unitId)?.linkedUnitIds?.length ?? 0;
    const covered = linked > 0 && units.length > 1 ? linked : 1;
    const start = b.checkIn > from ? b.checkIn : from;
    const end = b.checkOut < to ? b.checkOut : to;
    if (end > start) booked += nightsBetween(start, end) * covered;
  }
  return Math.min(1, booked / (units.length * nights));
}

export function nextRef(p: PropertyConfig, bookings: Booking[]): string {
  const prefix = p.name
    .split(/\s+/)
    .map((w) => w[0]?.toUpperCase() ?? "")
    .join("")
    .slice(0, 2)
    .padEnd(2, "X");
  const nums = bookings
    .filter((b) => b.ref.startsWith(prefix))
    .map((b) => Number(b.ref.split("-")[1]) || 0);
  return `${prefix}-${Math.max(2040, ...nums) + 1}`;
}

/** First night a unit is free from today, used for "Next arrival" on property cards. */
export function nextArrival(
  propertyId: string,
  bookings: Booking[],
  from = today(),
): Booking | undefined {
  return bookings
    .filter(
      (b) =>
        b.propertyId === propertyId &&
        b.checkIn >= from &&
        ["confirmed", "awaiting_payment"].includes(b.status),
    )
    .sort((a, b) => a.checkIn.localeCompare(b.checkIn))[0];
}

export const dayAfter = (iso: ISODate) => addDays(iso, 1);
