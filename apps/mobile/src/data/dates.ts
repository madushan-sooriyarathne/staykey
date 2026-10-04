import type { ISODate } from "./types";

/** Local-date helpers on "YYYY-MM-DD" strings, so stays never drift across time zones. */

export function toISO(d: Date): ISODate {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

export function fromISO(iso: ISODate): Date {
  const [y = 1970, m = 1, d = 1] = iso.split("-").map(Number);
  return new Date(y, m - 1, d);
}

export function today(): ISODate {
  return toISO(new Date());
}

export function addDays(iso: ISODate, days: number): ISODate {
  const d = fromISO(iso);
  d.setDate(d.getDate() + days);
  return toISO(d);
}

export function nightsBetween(from: ISODate, to: ISODate): number {
  return Math.round((fromISO(to).getTime() - fromISO(from).getTime()) / 86_400_000);
}

/** Every night of a stay, from check-in up to the night before check-out. */
export function eachNight(from: ISODate, to: ISODate): ISODate[] {
  const out: ISODate[] = [];
  for (let d = from; d < to; d = addDays(d, 1)) out.push(d);
  return out;
}

export function weekday(iso: ISODate): number {
  return fromISO(iso).getDay();
}

export function isWeekendNight(iso: ISODate): boolean {
  const w = weekday(iso);
  return w === 5 || w === 6;
}

const fmtShort = new Intl.DateTimeFormat("en-GB", {
  weekday: "short",
  day: "numeric",
  month: "short",
});
const fmtDay = new Intl.DateTimeFormat("en-GB", { day: "numeric", month: "short" });
const fmtMonth = new Intl.DateTimeFormat("en-GB", { month: "long", year: "numeric" });
const fmtMonthShort = new Intl.DateTimeFormat("en-GB", { month: "short" });
const fmtLong = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
});
const fmtTime = new Intl.DateTimeFormat("en-GB", {
  hour: "numeric",
  minute: "2-digit",
  hour12: true,
});

/** "Sun 4 Oct" */
export const formatShort = (iso: ISODate) => fmtShort.format(fromISO(iso)).replace(",", "");
/** "4 Oct" */
export const formatDay = (iso: ISODate) => fmtDay.format(fromISO(iso));
/** "October 2026" */
export const formatMonth = (iso: ISODate) => fmtMonth.format(fromISO(iso));
export const formatMonthShort = (iso: ISODate) => fmtMonthShort.format(fromISO(iso));
/** "Sunday, 4 October" */
export const formatLong = (iso: ISODate) => fmtLong.format(fromISO(iso));
/** "9:58 am" from an ISO timestamp. */
export const formatClock = (ts: string) => fmtTime.format(new Date(ts)).toUpperCase();

/** "4 to 8 Oct" or "30 Oct to 2 Nov". */
export function formatRange(from: ISODate, to: ISODate): string {
  const a = fromISO(from);
  const b = fromISO(to);
  if (a.getMonth() === b.getMonth() && a.getFullYear() === b.getFullYear()) {
    return `${a.getDate()} to ${fmtDay.format(b)}`;
  }
  return `${fmtDay.format(a)} to ${fmtDay.format(b)}`;
}

/** "MM-DD" season bounds as "15 Dec". */
export function formatMonthDay(md: string): string {
  const [m = 1, d = 1] = md.split("-").map(Number);
  return fmtDay.format(new Date(2026, m - 1, d));
}

export function monthStart(iso: ISODate): ISODate {
  return `${iso.slice(0, 7)}-01`;
}

export function addMonths(iso: ISODate, months: number): ISODate {
  const d = fromISO(monthStart(iso));
  d.setMonth(d.getMonth() + months);
  return toISO(d);
}

/** Six weeks of dates for a month grid, starting on Monday. */
export function monthGrid(month: ISODate): ISODate[] {
  const first = fromISO(monthStart(month));
  const offset = (first.getDay() + 6) % 7;
  const start = addDays(toISO(first), -offset);
  const days: ISODate[] = [];
  for (let i = 0; i < 42; i++) days.push(addDays(start, i));
  // Drop the last week when it belongs entirely to the next month.
  const lastWeek = days.slice(35);
  if (lastWeek.every((d) => d.slice(0, 7) !== month.slice(0, 7))) return days.slice(0, 35);
  return days;
}

export function inSameMonth(a: ISODate, b: ISODate): boolean {
  return a.slice(0, 7) === b.slice(0, 7);
}

/** Whether a night falls inside a "MM-DD" season, including seasons that wrap the year. */
export function inSeason(iso: ISODate, start: string, end: string): boolean {
  const md = iso.slice(5);
  return start <= end ? md >= start && md <= end : md >= start || md <= end;
}

export function relativeDay(iso: ISODate, base = today()): string {
  const diff = nightsBetween(base, iso);
  if (diff === 0) return "Today";
  if (diff === 1) return "Tomorrow";
  if (diff === -1) return "Yesterday";
  return formatShort(iso);
}

/** "Today", "Yesterday" or "Fri 2 Oct" for an ISO timestamp. */
export function dayLabel(ts: string): string {
  return relativeDay(toISO(new Date(ts)));
}
