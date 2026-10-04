/**
 * Domain shapes for the owner app. Money is always an integer in minor units next to an ISO
 * currency code, and calendar dates are local "YYYY-MM-DD" strings. These mirror what the API
 * will serve once bookings, units and rates move server side.
 */

export type Currency = "USD" | "LKR";
export type ISODate = string;

export type Unit = {
  id: string;
  name: string;
  sleeps: number;
  beds: string;
  /** Sunday to Thursday nightly rate. */
  rate: number;
  /** Friday and Saturday nightly rate. Falls back to `rate` when unset. */
  weekendRate?: number;
  /** A "whole house" unit books every listed room together, and each room blocks it. */
  linkedUnitIds?: string[];
};

export type Season = {
  id: string;
  name: string;
  /** Month and day, "MM-DD". Seasons may wrap the new year. */
  start: string;
  end: string;
  /** Nightly price per unit while the season applies. */
  prices: Record<string, number>;
  minNights?: number;
};

export type LengthDiscount = { nights: number; percent: number };

export type Charge = {
  id: string;
  name: string;
  kind: "percent" | "fixed";
  /** Percent as a whole number, or minor units for fixed charges. */
  amount: number;
  per: "stay" | "night" | "guest";
  enabled: boolean;
  note?: string;
};

export type Extra = {
  id: string;
  name: string;
  price: number;
  per: "stay" | "night" | "guest" | "trip" | "guestNight";
  onRequest: boolean;
  enabled: boolean;
};

export type Promo = {
  id: string;
  code: string;
  kind: "percent" | "fixed";
  amount: number;
  from?: ISODate;
  to?: ISODate;
  limit?: number;
  used: number;
  minNights?: number;
  note?: string;
};

export type IcalFeed = {
  id: string;
  channel: "airbnb" | "booking" | "agoda" | "expedia" | "other";
  url: string;
  status: "ok" | "error" | "pending";
  lastSync?: string;
  error?: string;
  upcoming: number;
};

export type Policy = "flexible" | "moderate" | "strict";

export type PropertyConfig = {
  id: string;
  name: string;
  slug: string;
  bookingPageUrl: string;
  bookingType: "entire" | "rooms";
  location: string;
  description: string;
  amenities: string[];
  checkIn: string;
  checkOut: string;
  photos: { uri: string; caption?: string }[];
  currency: Currency;

  units: Unit[];
  extraGuest: { above: number; amount: number } | null;
  seasons: Season[];
  lengthDiscounts: LengthDiscount[];

  rules: {
    minNights: number;
    maxNights: number;
    /** Same-day bookings close at this hour (24h). Null turns same-day bookings off. */
    sameDayCutoff: number | null;
    windowMonths: number;
    closedArrival: number[];
  };

  policy: Policy;
  depositPercent: number;
  balanceDueDays: number;
  houseRules: string[];

  charges: Charge[];
  extras: Extra[];
  promos: Promo[];

  payments: {
    bank: {
      enabled: boolean;
      bankName: string;
      accountName: string;
      accountNumber: string;
      payWithinHours: number;
      cancelIfUnpaid: boolean;
    };
    atProperty: boolean;
    cards: "off" | "pending" | "on";
  };

  booking: {
    mode: "instant" | "request";
    replyHours: number;
    holdMinutes: number;
    displayCurrencies: string[];
  };

  branding: { color: string; logoUri?: string };
  ical: IcalFeed[];
  icalExportToken: string;
};

export type Source =
  | "page"
  | "widget"
  | "whatsapp"
  | "phone"
  | "walkin"
  | "other"
  | "airbnb"
  | "booking"
  | "agoda"
  | "expedia";

export type BookingStatus =
  | "requested"
  | "awaiting_payment"
  | "confirmed"
  | "checked_in"
  | "checked_out"
  | "cancelled"
  | "declined";

export type PriceLine = { label: string; amount: number; kind?: "discount" | "charge" | "extra" };

export type Payment = {
  id: string;
  amount: number;
  method: "bank" | "cash" | "card" | "refund";
  at: string;
  note?: string;
};

export type Booking = {
  id: string;
  ref: string;
  propertyId: string;
  unitId: string;
  source: Source;
  status: BookingStatus;
  guest: { name: string; phone?: string; email?: string; country?: string };
  adults: number;
  children: number;
  checkIn: ISODate;
  checkOut: ISODate;
  lines: PriceLine[];
  total: number;
  payments: Payment[];
  slip?: { amount: number; at: string; status: "pending" | "accepted" | "rejected" };
  extras: string[];
  guestNote?: string;
  ownerNote?: string;
  createdAt: string;
  requestExpiresAt?: string;
  cancel?: { reason: string; refund: number; at: string };
};

export type Block = {
  id: string;
  propertyId: string;
  unitIds: string[];
  from: ISODate;
  /** Exclusive, like a check-out date. */
  to: ISODate;
  reason: "maintenance" | "owner" | "other";
  note?: string;
};

export type RateOverride = { price?: number; minNights?: number; closedToArrival?: boolean };

export type ActivityKind =
  | "request"
  | "booking"
  | "slip"
  | "payment"
  | "cancellation"
  | "sync"
  | "import";

export type Activity = {
  id: string;
  kind: ActivityKind;
  title: string;
  subtitle: string;
  at: string;
  read: boolean;
  bookingId?: string;
  propertyId?: string;
};

export type TeamMember = {
  id: string;
  name: string;
  phone: string;
  role: "owner" | "manager" | "caretaker";
  /** Empty means every property. */
  propertyIds: string[];
  status: "active" | "invited";
};

export type Template = {
  id: string;
  name: string;
  channels: ("email" | "whatsapp")[];
  timing: string;
  body: string;
};

export type NotificationEvent =
  | "booking"
  | "request"
  | "payment"
  | "cancellation"
  | "sync"
  | "summary";

export type Account = {
  name: string;
  photoUri?: string;
  phone: string;
  email: string;
  language: "en" | "si";
};
