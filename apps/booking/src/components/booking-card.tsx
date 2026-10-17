"use client";

// Dates, guests and the live price change as the guest picks, so the card runs in the browser.

import {
  type Currency,
  formatMoney,
  type PublicUnit,
  type StayRules,
  type UnitAvailability,
} from "@staykey/api-client";
import { useActionState, useId, useRef, useState, useTransition } from "react";
import { type FormState, hold, type QuoteState, quote } from "@/app/p/[slug]/actions";
import { addMonths, nightsBetween } from "@/lib/dates";
import { AvailabilityCalendar, type Range } from "./availability-calendar";

export type Stay = {
  unitId: string;
  checkIn: string;
  checkOut: string;
  adults: number;
  children: number;
};

type Props = {
  slug: string;
  currency: Currency;
  baseRate: number;
  units: PublicUnit[];
  availability: UnitAvailability[];
  rules: StayRules;
  today: string;
  initial?: Partial<Stay>;
  /** The price of the initial stay, worked out on the server. */
  initialQuote?: QuoteState;
  /** Where the guest came from, carried to the booking's source. */
  from?: "page" | "widget";
  compact?: boolean;
  /** Replaces holding the dates, as the embed does by opening the booking page instead. */
  onCheckAvailability?: (stay: Stay) => void;
};

/** Unit, dates and guests, with the price once dates are picked. Shared by the page and embed. */
export function BookingCard({
  slug,
  currency,
  baseRate,
  units,
  availability,
  rules,
  today,
  initial,
  initialQuote = {},
  from = "page",
  compact = false,
  onCheckAvailability,
}: Props) {
  const id = useId();
  const [unitId, setUnitId] = useState(
    units.some((u) => u.id === initial?.unitId) ? (initial?.unitId ?? "") : (units[0]?.id ?? ""),
  );
  const unit = units.find((u) => u.id === unitId);
  const days = availability.find((a) => a.unitId === unitId);
  const taken = new Set(days?.taken);
  const closed = new Set(days?.closedArrival);

  const [range, setRange] = useState<Range>(() => startRange(initial, taken));
  const [adults, setAdults] = useState(Math.max(1, initial?.adults ?? 2));
  const [children, setChildren] = useState(Math.max(0, initial?.children ?? 0));
  const sleeps = unit?.sleeps ?? 1;
  const ready = range.checkIn !== "" && range.checkOut !== "";
  const stay: Stay = { unitId, ...range, adults, children };

  // The price follows every change; only the newest answer is shown.
  const [priced, setPriced] = useState<QuoteState>(initialQuote);
  const [pricing, startPricing] = useTransition();
  const latest = useRef(0);
  const change = (next: Partial<Stay>) => {
    const s = { ...stay, ...next };
    setUnitId(s.unitId);
    setRange({ checkIn: s.checkIn, checkOut: s.checkOut });
    setAdults(s.adults);
    setChildren(s.children);
    const ask = ++latest.current;
    if (!s.checkIn || !s.checkOut) {
      setPriced({});
      return;
    }
    startPricing(async () => {
      const res = await quote({ slug, ...s });
      if (ask === latest.current) setPriced(res);
    });
  };

  const [state, holdAction, holding] = useActionState<FormState, FormData>(hold, {});

  return (
    <form
      action={onCheckAvailability ? undefined : holdAction}
      onSubmit={(event) => {
        if (!onCheckAvailability) return;
        event.preventDefault();
        if (ready) onCheckAvailability(stay);
      }}
      className="flex flex-col gap-4 rounded-card border border-cloud bg-snow p-5"
    >
      <input type="hidden" name="slug" value={slug} />
      <input type="hidden" name="unitId" value={unitId} />
      <input type="hidden" name="checkIn" value={range.checkIn} />
      <input type="hidden" name="checkOut" value={range.checkOut} />
      <input type="hidden" name="from" value={from} />

      {!compact && (
        <p className="text-steel">
          From{" "}
          <span className="font-semibold text-obsidian text-xl">
            {formatMoney(baseRate, currency)}
          </span>{" "}
          a night
        </p>
      )}

      {units.length > 1 && (
        <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={`${id}-unit`}>
          Room
          <select
            id={`${id}-unit`}
            value={unitId}
            onChange={(e) => change({ unitId: e.target.value, checkIn: "", checkOut: "" })}
            className="h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus-visible:border-ember"
          >
            {units.map((u) => (
              <option key={u.id} value={u.id}>
                {u.name}, sleeps {u.sleeps}
              </option>
            ))}
          </select>
        </label>
      )}

      <AvailabilityCalendar
        key={unitId}
        today={today}
        lastDay={addMonths(today, rules.windowMonths)}
        taken={taken}
        closedArrival={closed}
        value={range}
        onChange={(r) => change(r)}
      />

      <div className="grid grid-cols-2 gap-2">
        <GuestSelect
          id={`${id}-adults`}
          name="adults"
          label="Adults"
          value={adults}
          min={1}
          max={sleeps}
          onChange={(n) => change({ adults: n, children: Math.min(children, sleeps - n) })}
        />
        <GuestSelect
          id={`${id}-children`}
          name="children"
          label="Children"
          value={children}
          min={0}
          max={Math.max(0, sleeps - adults)}
          onChange={(n) => change({ children: n })}
        />
      </div>

      {ready && (
        <div className="flex items-baseline justify-between text-graphite" aria-live="polite">
          {priced.problem ? (
            <p className="text-ember-ink text-sm">{priced.problem.message}</p>
          ) : (
            <>
              <span>
                {nightsBetween(range.checkIn, range.checkOut)} nights
                {pricing && <span className="text-fog"> · pricing</span>}
              </span>
              {priced.quote && (
                <span className="font-semibold text-obsidian">
                  {formatMoney(priced.quote.total, currency)}
                </span>
              )}
            </>
          )}
        </div>
      )}

      <button
        type="submit"
        disabled={!ready || holding || Boolean(priced.problem)}
        className="h-12 rounded-button bg-obsidian font-medium text-snow shadow-button transition-opacity disabled:opacity-40"
      >
        {holding ? "Holding your dates" : onCheckAvailability ? "Check availability" : "Continue"}
      </button>
      {state.problem && (
        <p role="alert" className="text-ember-ink text-sm">
          {state.problem.message}
        </p>
      )}
    </form>
  );
}

type GuestSelectProps = {
  id: string;
  name: string;
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (n: number) => void;
};

function GuestSelect({ id, name, label, value, min, max, onChange }: GuestSelectProps) {
  const options = Array.from({ length: Math.max(0, max - min) + 1 }, (_, i) => min + i);
  return (
    <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={id}>
      {label}
      <select
        id={id}
        name={name}
        value={Math.min(value, max)}
        onChange={(e) => onChange(Number(e.target.value))}
        className="h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus-visible:border-ember"
      >
        {options.map((n) => (
          <option key={n} value={n}>
            {n}
          </option>
        ))}
      </select>
    </label>
  );
}

/** The dates the guest arrived with, unless they cross a night that's taken. */
function startRange(initial: Partial<Stay> | undefined, taken: ReadonlySet<string>): Range {
  const checkIn = initial?.checkIn ?? "";
  const checkOut = initial?.checkOut ?? "";
  if (!checkIn || !checkOut || checkOut <= checkIn) return { checkIn: "", checkOut: "" };
  for (const night of taken) {
    if (night >= checkIn && night < checkOut) return { checkIn: "", checkOut: "" };
  }
  return { checkIn, checkOut };
}
