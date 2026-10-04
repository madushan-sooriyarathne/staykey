import type { Account, Currency, NotificationEvent, PropertyConfig, Template, Unit } from "./types";

let counter = 0;
/** Short unique id for records created on the device. */
export function uid(prefix = "id"): string {
  counter = (counter + 1) % 1000;
  return `${prefix}_${Date.now().toString(36)}${counter.toString(36)}${Math.random().toString(36).slice(2, 6)}`;
}

/** Converts a US dollar figure into minor units of the property's currency, for sample values. */
export function sampleMoney(usd: number, currency: Currency): number {
  return currency === "USD" ? Math.round(usd * 100) : Math.round(usd * 300) * 100;
}

export const AMENITIES = [
  "Pool",
  "WiFi",
  "Air conditioning",
  "Kitchen",
  "Parking",
  "Hot water",
  "Garden",
  "Beach access",
  "Breakfast",
  "Washing machine",
];

export const BRAND_COLORS = ["#09090b", "#ff5a00", "#0f766e", "#1d4ed8", "#7c3aed", "#be123c"];

type Base = Pick<
  PropertyConfig,
  "id" | "name" | "slug" | "bookingPageUrl" | "bookingType" | "currency"
> &
  Partial<PropertyConfig> & { units: Unit[] };

/** A complete property with sensible defaults for everything onboarding didn't ask. */
export function makeProperty(base: Base): PropertyConfig {
  return {
    location: "",
    description: "",
    amenities: [],
    checkIn: "14:00",
    checkOut: "11:00",
    photos: [],
    extraGuest: null,
    seasons: [],
    lengthDiscounts: [],
    rules: { minNights: 1, maxNights: 30, sameDayCutoff: 12, windowMonths: 12, closedArrival: [] },
    policy: "moderate",
    depositPercent: 30,
    balanceDueDays: 14,
    houseRules: ["No smoking indoors", "No parties or events"],
    charges: [
      {
        id: uid("chg"),
        name: "Service charge",
        kind: "percent",
        amount: 10,
        per: "stay",
        enabled: false,
        note: "on the room rate",
      },
      {
        id: uid("chg"),
        name: "VAT",
        kind: "percent",
        amount: 18,
        per: "stay",
        enabled: false,
        note: "on the room rate, once you register",
      },
    ],
    extras: [],
    promos: [],
    payments: {
      bank: {
        enabled: true,
        bankName: "",
        accountName: "",
        accountNumber: "",
        payWithinHours: 24,
        cancelIfUnpaid: true,
      },
      atProperty: true,
      cards: "off",
    },
    booking: {
      mode: "instant",
      replyHours: 24,
      holdMinutes: 15,
      displayCurrencies: [base.currency],
    },
    branding: { color: "#09090b" },
    ical: [],
    icalExportToken: Math.random().toString(16).slice(2, 8),
    ...base,
  };
}

export const DEFAULT_TEMPLATES: Template[] = [
  {
    id: "confirmed",
    name: "Booking confirmed",
    channels: ["email"],
    timing: "Right after confirmation",
    body: "Hi {guest_name}, your stay at {property_name} is confirmed for {check_in} to {check_out}. We can't wait to host you.",
  },
  {
    id: "reminder",
    name: "Payment reminder",
    channels: ["email", "whatsapp"],
    timing: "A day before it's due",
    body: "Hi {guest_name}, a friendly reminder that {balance} is due for your stay at {property_name}. You can pay here: {booking_link}",
  },
  {
    id: "prearrival",
    name: "Pre-arrival info",
    channels: ["email", "whatsapp"],
    timing: "2 days before check-in",
    body: "Hi {guest_name}, we look forward to welcoming you on {check_in}. Here is the map pin: {map_link}. Check-in is from {check_in_time}.",
  },
  {
    id: "thanks",
    name: "Thank you",
    channels: ["email"],
    timing: "The day after check-out",
    body: "Thank you for staying at {property_name}, {guest_name}. Book directly next time and we'll take care of the rest: {booking_link}",
  },
];

export const TEMPLATE_VARIABLES = [
  { key: "guest_name", label: "Guest name" },
  { key: "property_name", label: "Property name" },
  { key: "check_in", label: "Check-in date" },
  { key: "check_out", label: "Check-out date" },
  { key: "check_in_time", label: "Check-in time" },
  { key: "balance", label: "Balance due" },
  { key: "map_link", label: "Map link" },
  { key: "booking_link", label: "Booking link" },
];

export const NOTIFICATION_EVENTS: { id: NotificationEvent; label: string; hint?: string }[] = [
  { id: "booking", label: "New booking" },
  { id: "request", label: "Booking request" },
  { id: "payment", label: "Payment or slip" },
  { id: "cancellation", label: "Cancellation" },
  { id: "sync", label: "Sync problem" },
  { id: "summary", label: "Morning summary", hint: "Every day at 7:00 AM" },
];

export const DEFAULT_NOTIFICATIONS: Record<
  NotificationEvent,
  { push: boolean; whatsapp: boolean }
> = {
  booking: { push: true, whatsapp: true },
  request: { push: true, whatsapp: true },
  payment: { push: true, whatsapp: false },
  cancellation: { push: true, whatsapp: false },
  sync: { push: true, whatsapp: false },
  summary: { push: false, whatsapp: true },
};

export const DEFAULT_ACCOUNT: Account = { name: "", phone: "", email: "", language: "en" };
