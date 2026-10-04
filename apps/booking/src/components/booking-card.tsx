"use client";

import { type Currency, formatMoney } from "@staykey/api-client";
import { useId, useState } from "react";

type Props = {
  baseRate: number;
  currency: Currency;
  compact?: boolean;
  onCheckAvailability?: (dates: { checkIn: string; checkOut: string; guests: number }) => void;
};

/**
 * Date and guest picker shared by the hosted page and the embed.
 * Availability and checkout are not wired yet; the button reports the selection.
 */
export function BookingCard({ baseRate, currency, compact = false, onCheckAvailability }: Props) {
  const id = useId();
  const [checkIn, setCheckIn] = useState("");
  const [checkOut, setCheckOut] = useState("");
  const [guests, setGuests] = useState(2);
  const ready = checkIn !== "" && checkOut !== "" && checkOut > checkIn;

  return (
    <form
      className="flex flex-col gap-3 rounded-card border border-cloud bg-snow p-5"
      onSubmit={(event) => {
        event.preventDefault();
        if (ready) onCheckAvailability?.({ checkIn, checkOut, guests });
      }}
    >
      {!compact && (
        <p className="text-steel">
          From{" "}
          <span className="font-semibold text-obsidian text-xl">
            {formatMoney(baseRate, currency)}
          </span>{" "}
          a night
        </p>
      )}
      <div className="grid grid-cols-2 gap-2">
        <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={`${id}-in`}>
          Check-in
          <input
            id={`${id}-in`}
            type="date"
            value={checkIn}
            onChange={(e) => setCheckIn(e.target.value)}
            className="h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus:border-ember focus:ring-3 focus:ring-ember-tint"
          />
        </label>
        <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={`${id}-out`}>
          Check-out
          <input
            id={`${id}-out`}
            type="date"
            value={checkOut}
            min={checkIn || undefined}
            onChange={(e) => setCheckOut(e.target.value)}
            className="h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus:border-ember focus:ring-3 focus:ring-ember-tint"
          />
        </label>
      </div>
      <label className="flex items-center justify-between text-graphite" htmlFor={`${id}-guests`}>
        Guests
        <input
          id={`${id}-guests`}
          type="number"
          min={1}
          max={20}
          value={guests}
          onChange={(e) => setGuests(Number(e.target.value))}
          className="h-10 w-20 rounded-input border border-cloud bg-snow px-3 text-right outline-none focus:border-ember"
        />
      </label>
      <button
        type="submit"
        disabled={!ready}
        className="h-12 rounded-button bg-obsidian font-medium text-snow shadow-button transition-opacity disabled:opacity-40"
      >
        Check availability
      </button>
    </form>
  );
}
