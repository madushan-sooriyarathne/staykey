import type { Block, Extra, IcalFeed, Payment } from "./types";

export const CHANNEL_LABEL: Record<IcalFeed["channel"], string> = {
  airbnb: "Airbnb",
  booking: "Booking.com",
  agoda: "Agoda",
  expedia: "Expedia",
  other: "Other calendar",
};

export const METHOD_LABEL: Record<Payment["method"], string> = {
  bank: "Bank transfer",
  cash: "Cash",
  card: "Card",
  refund: "Refund",
};

export const BLOCK_REASON: Record<Block["reason"], string> = {
  maintenance: "Maintenance",
  owner: "Owner stay",
  other: "Other",
};

export const EXTRA_PER: Record<Extra["per"], string> = {
  stay: "per stay",
  trip: "per trip",
  night: "per night",
  guest: "per guest",
  guestNight: "per guest per night",
};

/** "2:00 PM" from "14:00". */
export function clock(hhmm: string): string {
  const [h = 0, m = 0] = hhmm.split(":").map(Number);
  const hour = h % 12 === 0 ? 12 : h % 12;
  return `${hour}:${String(m).padStart(2, "0")} ${h >= 12 ? "PM" : "AM"}`;
}

export function plural(n: number, one: string, many = `${one}s`): string {
  return `${n} ${n === 1 ? one : many}`;
}
