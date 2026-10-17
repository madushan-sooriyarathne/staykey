"use client";

// Guest details, extras and the promo code re-price the stay as the guest types.

import {
  type Currency,
  formatMoney,
  type PublicExtra,
  type PublicProperty,
} from "@staykey/api-client";
import Link from "next/link";
import { useActionState, useEffect, useId, useRef, useState, useTransition } from "react";
import { book, type FormState, type QuoteState, quote } from "../actions";

type Props = {
  slug: string;
  back: string;
  stay: { unitId: string; checkIn: string; checkOut: string; adults: number; children: number };
  sleeps: number;
  holdId?: string;
  expiresAt?: string;
  from: "page" | "widget";
  currency: Currency;
  extras: PublicExtra[];
  payments: PublicProperty["payments"];
  mode: PublicProperty["booking"]["mode"];
  initialQuote: QuoteState;
};

const field =
  "h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus-visible:border-ember";

export function CheckoutForm(props: Props) {
  const { slug, back, stay, sleeps, holdId, expiresAt, from, currency, extras, payments, mode } =
    props;
  const id = useId();
  // One key per form, so a retried submit can't book twice.
  const [key] = useState(() => crypto.randomUUID());
  const [adults, setAdults] = useState(stay.adults);
  const [children, setChildren] = useState(stay.children);
  const [chosen, setChosen] = useState<string[]>([]);
  const [promo, setPromo] = useState("");
  const [applied, setApplied] = useState("");
  const [priced, setPriced] = useState(props.initialQuote);
  const [pricing, startPricing] = useTransition();
  const latest = useRef(0);
  const [state, bookAction, booking] = useActionState<FormState, FormData>(book, {});
  const lapsed = useLapsed(expiresAt);

  const reprice = (next: {
    adults?: number;
    children?: number;
    extras?: string[];
    promo?: string;
  }) => {
    const ask = ++latest.current;
    startPricing(async () => {
      const res = await quote({
        slug,
        ...stay,
        adults: next.adults ?? adults,
        children: next.children ?? children,
        extras: next.extras ?? chosen,
        promo: next.promo ?? applied,
      });
      if (ask === latest.current) setPriced(res);
    });
  };

  const methods = [
    payments.bank && { value: "bank", label: "Bank transfer", note: "Bank details come next." },
    payments.atProperty && { value: "property", label: "Pay at the property", note: "On arrival." },
  ].filter((m) => m !== false);

  const problemFor = (name: string) =>
    state.problem?.field === name || state.problem?.field === `guest.${name}`
      ? state.problem.message
      : undefined;

  return (
    <form action={bookAction} className="flex flex-col gap-6">
      <input type="hidden" name="slug" value={slug} />
      <input type="hidden" name="key" value={key} />
      <input type="hidden" name="holdId" value={holdId ?? ""} />
      <input type="hidden" name="unitId" value={stay.unitId} />
      <input type="hidden" name="checkIn" value={stay.checkIn} />
      <input type="hidden" name="checkOut" value={stay.checkOut} />
      <input type="hidden" name="from" value={from} />
      <input type="hidden" name="promo" value={applied} />

      {lapsed ? (
        <p role="status" className="rounded-card bg-ember-tint p-4 text-ember-ink">
          We've stopped holding these dates. You can still book if they're free, or{" "}
          <Link href={back} className="underline">
            pick dates again
          </Link>
          .
        </p>
      ) : (
        expiresAt && <HoldTimer expiresAt={expiresAt} />
      )}

      <fieldset className="flex flex-col gap-3">
        <legend className="mb-2 font-semibold text-obsidian text-xl">Who's coming</legend>
        <Text
          id={`${id}-name`}
          name="name"
          label="Full name"
          autoComplete="name"
          required
          problem={problemFor("name")}
        />
        <Text
          id={`${id}-phone`}
          name="phone"
          label="Phone (WhatsApp if you have it)"
          type="tel"
          autoComplete="tel"
          required
          problem={problemFor("phone")}
        />
        <Text
          id={`${id}-email`}
          name="email"
          label="Email (optional)"
          type="email"
          autoComplete="email"
          problem={problemFor("email")}
        />
        <Text
          id={`${id}-country`}
          name="country"
          label="Country code (optional, such as GB)"
          autoComplete="country"
          maxLength={2}
          problem={problemFor("country")}
        />
        <div className="grid grid-cols-2 gap-2">
          <Count
            id={`${id}-adults`}
            name="adults"
            label="Adults"
            value={adults}
            min={1}
            max={sleeps}
            onChange={(n) => {
              const c = Math.min(children, sleeps - n);
              setAdults(n);
              setChildren(c);
              reprice({ adults: n, children: c });
            }}
          />
          <Count
            id={`${id}-children`}
            name="children"
            label="Children"
            value={children}
            min={0}
            max={sleeps - adults}
            onChange={(n) => {
              setChildren(n);
              reprice({ children: n });
            }}
          />
        </div>
      </fieldset>

      {extras.length > 0 && (
        <fieldset className="flex flex-col gap-2">
          <legend className="mb-2 font-semibold text-obsidian text-xl">Extras</legend>
          {extras.map((e) => (
            <label key={e.id} className="flex items-center justify-between gap-3 text-graphite">
              <span className="flex items-center gap-3">
                <input
                  type="checkbox"
                  name="extras"
                  value={e.id}
                  checked={chosen.includes(e.id)}
                  onChange={(ev) => {
                    const next = ev.target.checked
                      ? [...chosen, e.id]
                      : chosen.filter((x) => x !== e.id);
                    setChosen(next);
                    reprice({ extras: next });
                  }}
                  className="h-5 w-5 accent-obsidian"
                />
                {e.name}
                {e.onRequest && <span className="text-fog text-sm">on request</span>}
              </span>
              <span className="text-steel">{formatMoney(e.price, currency)}</span>
            </label>
          ))}
        </fieldset>
      )}

      <div className="flex flex-col gap-1.5">
        <label className="text-sm text-steel" htmlFor={`${id}-promo`}>
          Promo code
        </label>
        <div className="flex gap-2">
          <input
            id={`${id}-promo`}
            value={promo}
            onChange={(e) => setPromo(e.target.value.toUpperCase())}
            maxLength={40}
            className={`${field} flex-1`}
          />
          <button
            type="button"
            onClick={() => {
              setApplied(promo);
              reprice({ promo });
            }}
            className="h-12 rounded-button border border-cloud px-4 text-graphite hover:bg-cloud"
          >
            Apply
          </button>
        </div>
      </div>

      <fieldset className="flex flex-col gap-2">
        <legend className="mb-2 font-semibold text-obsidian text-xl">How you'll pay</legend>
        {methods.map((m, i) => (
          <label
            key={m.value}
            className="flex items-start gap-3 rounded-card border border-cloud bg-snow p-4 text-graphite"
          >
            <input
              type="radio"
              name="paymentMethod"
              value={m.value}
              defaultChecked={i === 0}
              required
              className="mt-1 h-5 w-5 accent-obsidian"
            />
            <span className="flex flex-col">
              {m.label}
              <span className="text-sm text-steel">{m.note}</span>
            </span>
          </label>
        ))}
      </fieldset>

      <div className="flex flex-col gap-1.5">
        <label className="text-sm text-steel" htmlFor={`${id}-note`}>
          Anything the owner should know (optional)
        </label>
        <textarea
          id={`${id}-note`}
          name="note"
          rows={3}
          maxLength={2000}
          className={`${field} h-auto py-3`}
        />
      </div>

      <Summary priced={priced} pricing={pricing} currency={currency} />

      <button
        type="submit"
        disabled={booking || !priced.quote}
        className="h-12 rounded-button bg-obsidian font-medium text-snow shadow-button transition-opacity disabled:opacity-40"
      >
        {booking ? "Booking" : mode === "request" ? "Send booking request" : "Book now"}
      </button>
      {state.problem && (
        <p role="alert" className="text-ember-ink">
          {state.problem.message}
        </p>
      )}
      {mode === "request" && (
        <p className="text-center text-sm text-steel">
          The owner confirms requests, usually within a day. You pay once they do.
        </p>
      )}
    </form>
  );
}

function Summary({
  priced,
  pricing,
  currency,
}: {
  priced: QuoteState;
  pricing: boolean;
  currency: Currency;
}) {
  if (priced.problem) {
    return (
      <p role="alert" className="text-ember-ink">
        {priced.problem.message}
      </p>
    );
  }
  if (!priced.quote) return null;
  const q = priced.quote;
  return (
    <section aria-live="polite" className="flex flex-col gap-2 rounded-card bg-subtle p-4">
      {q.lines.map((l) => (
        <p key={l.label} className="flex justify-between text-graphite">
          <span>{l.label}</span>
          <span>{formatMoney(l.amount, currency)}</span>
        </p>
      ))}
      <p className="flex justify-between border-cloud border-t pt-2 font-semibold text-obsidian">
        <span>Total{pricing && <span className="font-normal text-fog"> · updating</span>}</span>
        <span>{formatMoney(q.total, currency)}</span>
      </p>
      {q.deposit > 0 && q.deposit < q.total && (
        <p className="flex justify-between text-sm text-steel">
          <span>Deposit to secure it</span>
          <span>{formatMoney(q.deposit, currency)}</span>
        </p>
      )}
    </section>
  );
}

type TextProps = {
  id: string;
  name: string;
  label: string;
  type?: string;
  autoComplete?: string;
  required?: boolean;
  maxLength?: number;
  problem?: string;
};

function Text({
  id,
  name,
  label,
  type = "text",
  autoComplete,
  required,
  maxLength,
  problem,
}: TextProps) {
  return (
    <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={id}>
      {label}
      <input
        id={id}
        name={name}
        type={type}
        autoComplete={autoComplete}
        required={required}
        maxLength={maxLength ?? 254}
        aria-invalid={problem ? true : undefined}
        aria-describedby={problem ? `${id}-problem` : undefined}
        className={field}
      />
      {problem && (
        <span id={`${id}-problem`} className="text-ember-ink">
          {problem}
        </span>
      )}
    </label>
  );
}

type CountProps = {
  id: string;
  name: string;
  label: string;
  value: number;
  min: number;
  max: number;
  onChange: (n: number) => void;
};

function Count({ id, name, label, value, min, max, onChange }: CountProps) {
  const options = Array.from({ length: Math.max(0, max - min) + 1 }, (_, i) => min + i);
  return (
    <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={id}>
      {label}
      <select
        id={id}
        name={name}
        value={value}
        onChange={(e) => onChange(Number(e.target.value))}
        className={field}
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

/** Minutes and seconds left on the hold, ticking each second. */
function HoldTimer({ expiresAt }: { expiresAt: string }) {
  const left = useSecondsLeft(expiresAt);
  if (left === null) return null;
  const m = Math.floor(left / 60);
  const s = String(left % 60).padStart(2, "0");
  return (
    <p className="text-sm text-steel">
      We're holding these dates for you for {m}:{s}.
    </p>
  );
}

function useLapsed(expiresAt?: string): boolean {
  const left = useSecondsLeft(expiresAt);
  return left === 0;
}

// The countdown follows the clock, an outside source, so it lives in an effect with a timer.
function useSecondsLeft(expiresAt?: string): number | null {
  const [left, setLeft] = useState<number | null>(null);
  useEffect(() => {
    if (!expiresAt) return;
    const end = Date.parse(expiresAt);
    if (Number.isNaN(end)) return;
    const tick = () => setLeft(Math.max(0, Math.ceil((end - Date.now()) / 1000)));
    tick();
    const timer = setInterval(tick, 1000);
    return () => clearInterval(timer);
  }, [expiresAt]);
  return left;
}
