"use client";

// Picking dates is click and keyboard state, so the calendar runs in the browser.

import { useState } from "react";
import { cx } from "@/lib/cx";
import { addDays, addMonths, formatDay, formatMonth } from "@/lib/dates";

export type Range = { checkIn: string; checkOut: string };

type Props = {
  /** The property's today. Earlier days can't be picked. */
  today: string;
  /** The last day a stay may end on, from the property's booking window. */
  lastDay: string;
  taken: ReadonlySet<string>;
  closedArrival: ReadonlySet<string>;
  value: Range;
  onChange: (range: Range) => void;
};

const WEEKDAYS = ["Mon", "Tue", "Wed", "Thu", "Fri", "Sat", "Sun"];

/**
 * Two months of days. The first pick is the check-in, the second the check-out; a range can't
 * cross a taken night, though a stay may end on the morning a taken night begins.
 */
export function AvailabilityCalendar({
  today,
  lastDay,
  taken,
  closedArrival,
  value,
  onChange,
}: Props) {
  const [first, setFirst] = useState(() => monthStart(value.checkIn || today));
  const picking = value.checkIn !== "" && value.checkOut === "";

  const canArrive = (day: string) =>
    day >= today && day < lastDay && !taken.has(day) && !closedArrival.has(day);
  const canLeave = (day: string) => {
    if (!picking || day <= value.checkIn || day > lastDay) return false;
    for (let night = value.checkIn; night < day; night = addDays(night, 1)) {
      if (taken.has(night)) return false;
    }
    return true;
  };

  const pick = (day: string) => {
    if (canLeave(day)) onChange({ checkIn: value.checkIn, checkOut: day });
    else if (canArrive(day)) onChange({ checkIn: day, checkOut: "" });
  };

  const earliest = monthStart(today);
  const latest = monthStart(lastDay);
  return (
    <div className="@container flex flex-col gap-3">
      <div className="flex items-center justify-between">
        <button
          type="button"
          onClick={() => setFirst(addMonths(first, -1))}
          disabled={first <= earliest}
          aria-label="Earlier months"
          className="h-9 w-9 rounded-button text-graphite hover:bg-cloud disabled:opacity-30"
        >
          ‹
        </button>
        <p className="text-sm text-steel" aria-live="polite">
          {hint(value, picking)}
        </p>
        <button
          type="button"
          onClick={() => setFirst(addMonths(first, 1))}
          disabled={addMonths(first, 1) >= latest}
          aria-label="Later months"
          className="h-9 w-9 rounded-button text-graphite hover:bg-cloud disabled:opacity-30"
        >
          ›
        </button>
      </div>
      <div className="grid gap-6 @lg:grid-cols-2">
        {[first, addMonths(first, 1)].map((month) => (
          <Month
            key={month}
            month={month}
            value={value}
            isTaken={(day) => taken.has(day)}
            isOpen={(day) => canLeave(day) || canArrive(day)}
            isClosed={(day) => closedArrival.has(day)}
            onPick={pick}
          />
        ))}
      </div>
    </div>
  );
}

type MonthProps = {
  month: string;
  value: Range;
  isTaken: (day: string) => boolean;
  isOpen: (day: string) => boolean;
  isClosed: (day: string) => boolean;
  onPick: (day: string) => void;
};

function Month({ month, value, isTaken, isOpen, isClosed, onPick }: MonthProps) {
  const title = formatMonth(month);
  const lead = (new Date(`${month}T00:00:00Z`).getUTCDay() + 6) % 7;
  const days: string[] = [];
  for (let day = month; day.slice(0, 7) === month.slice(0, 7); day = addDays(day, 1))
    days.push(day);

  return (
    <div>
      <p className="mb-2 text-center font-medium text-graphite">{title}</p>
      <div className="grid grid-cols-7 gap-y-1 text-center">
        {WEEKDAYS.map((w) => (
          <span key={w} className="text-caption text-fog">
            {w}
          </span>
        ))}
        {days.map((day) => {
          const end = day === value.checkIn || day === value.checkOut;
          const inside = value.checkOut !== "" && day > value.checkIn && day < value.checkOut;
          const taken = isTaken(day);
          const open = isOpen(day);
          return (
            <button
              key={day}
              type="button"
              style={day === month ? { gridColumnStart: lead + 1 } : undefined}
              disabled={!open}
              onClick={() => onPick(day)}
              aria-pressed={end}
              aria-label={`${formatDay(day)}, ${state(day, value, taken, isClosed(day), open)}`}
              className={cx(
                "mx-auto aspect-square w-full max-w-11 rounded-full text-sm transition-colors",
                end && "bg-obsidian font-medium text-snow",
                inside && "bg-cloud text-graphite",
                !end && !inside && open && "text-graphite hover:bg-cloud",
                !open && "text-ash",
                taken && !end && "line-through",
                "focus-visible:outline-2 focus-visible:outline-ember",
              )}
            >
              {Number(day.slice(8))}
            </button>
          );
        })}
      </div>
    </div>
  );
}

function monthStart(day: string): string {
  return `${day.slice(0, 7)}-01`;
}

function hint(value: Range, picking: boolean): string {
  if (picking) return "Now pick your check-out";
  if (value.checkIn && value.checkOut) return "Pick a new check-in to change";
  return "Pick your check-in";
}

function state(day: string, value: Range, taken: boolean, closed: boolean, open: boolean): string {
  if (day === value.checkIn) return "check-in";
  if (day === value.checkOut) return "check-out";
  if (taken) return open ? "taken, can be your check-out" : "taken";
  if (closed && !open) return "no arrivals";
  return open ? "available" : "unavailable";
}
