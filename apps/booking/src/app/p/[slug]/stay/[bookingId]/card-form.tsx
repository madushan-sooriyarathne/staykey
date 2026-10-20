"use client";

// The pending state and the problem message follow the checkout, which opens from a form action.

import { type Currency, formatMoney, type GuestCard } from "@staykey/api-client";
import { useActionState } from "react";
import { type FormState, payByCard } from "../../actions";

type Props = { bookingId: string; token: string; currency: Currency; card: GuestCard };

const LABEL = {
  deposit: "Pay the deposit by card",
  full: "Pay in full by card",
  balance: "Pay the balance by card",
} as const;

export function CardForm({ bookingId, token, currency, card }: Props) {
  const [state, action, opening] = useActionState<FormState, FormData>(payByCard, {});
  return (
    <form
      action={action}
      className="flex flex-col gap-3 rounded-card border border-cloud bg-snow p-5"
    >
      <h2 className="font-semibold text-obsidian text-xl">Pay by card</h2>
      <input type="hidden" name="bookingId" value={bookingId} />
      <input type="hidden" name="token" value={token} />
      {card.options.map((o) => (
        <button
          key={o.purpose}
          type="submit"
          name="purpose"
          value={o.purpose}
          disabled={opening}
          className="flex h-auto min-h-12 flex-col items-center justify-center rounded-button bg-obsidian px-4 py-2 font-medium text-snow shadow-button transition-opacity disabled:opacity-40"
        >
          <span>{LABEL[o.purpose]}</span>
          <span className="font-normal text-sm opacity-80">
            {formatMoney(o.charged, "LKR")}
            {currency !== "LKR" && ` (${formatMoney(o.amount, currency)})`}
          </span>
        </button>
      ))}
      <p className="text-sm text-steel">
        You pay on Payments.lk's secure page, in Sri Lankan rupees.
        {card.testMode && " Test mode, no real charge."}
      </p>
      {state.problem && (
        <p role="alert" className="text-ember-ink text-sm">
          {state.problem.message}
        </p>
      )}
    </form>
  );
}
