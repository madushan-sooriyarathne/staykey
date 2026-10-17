/** Calendar days as YYYY-MM-DD strings, the format the API uses for stays. */

export function addDays(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCDate(d.getUTCDate() + n);
  return d.toISOString().slice(0, 10);
}

export function addMonths(day: string, n: number): string {
  const d = new Date(`${day}T00:00:00Z`);
  d.setUTCMonth(d.getUTCMonth() + n);
  return d.toISOString().slice(0, 10);
}

export function nightsBetween(from: string, to: string): number {
  return Math.round((Date.parse(`${to}T00:00:00Z`) - Date.parse(`${from}T00:00:00Z`)) / 86_400_000);
}

/** Today in a time zone, such as the property's. */
export function todayIn(timeZone: string): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone }).format(new Date());
}

// Names are spelled out rather than taken from Intl, whose output differs between the server
// and browsers ("Wed 2 Dec" against "Wed, 2 Dec") and would break hydration.
const WEEKDAYS = ["Sun", "Mon", "Tue", "Wed", "Thu", "Fri", "Sat"];
const MONTHS = [
  "January",
  "February",
  "March",
  "April",
  "May",
  "June",
  "July",
  "August",
  "September",
  "October",
  "November",
  "December",
];

const short = (d: Date) => MONTHS[d.getUTCMonth()].slice(0, 3);

/** "Mon 10 Nov" */
export function formatDay(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  return `${WEEKDAYS[d.getUTCDay()]} ${d.getUTCDate()} ${short(d)}`;
}

/** "November 2026" */
export function formatMonth(day: string): string {
  const d = new Date(`${day}T00:00:00Z`);
  return `${MONTHS[d.getUTCMonth()]} ${d.getUTCFullYear()}`;
}

/** "10 to 13 Nov", or "30 Oct to 2 Nov" across months, as the owner app writes stays. */
export function formatRange(from: string, to: string): string {
  const a = new Date(`${from}T00:00:00Z`);
  const b = new Date(`${to}T00:00:00Z`);
  if (a.getUTCMonth() === b.getUTCMonth() && a.getUTCFullYear() === b.getUTCFullYear()) {
    return `${a.getUTCDate()} to ${b.getUTCDate()} ${short(b)}`;
  }
  return `${a.getUTCDate()} ${short(a)} to ${b.getUTCDate()} ${short(b)}`;
}

export function isDay(value: unknown): value is string {
  return typeof value === "string" && /^\d{4}-\d{2}-\d{2}$/.test(value);
}
