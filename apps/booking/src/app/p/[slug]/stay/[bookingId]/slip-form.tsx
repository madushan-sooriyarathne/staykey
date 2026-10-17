"use client";

// The pending state and the problem message follow the upload, which runs as a form action.

import type { Currency } from "@staykey/api-client";
import { useActionState, useId } from "react";
import { type FormState, sendSlip } from "../../actions";

type Props = { bookingId: string; token: string; currency: Currency };

export function SlipForm({ bookingId, token, currency }: Props) {
  const id = useId();
  const [state, action, sending] = useActionState<FormState, FormData>(sendSlip, {});
  return (
    <form
      action={action}
      className="flex flex-col gap-3 rounded-card border border-cloud bg-snow p-5"
    >
      <h2 className="font-semibold text-obsidian text-xl">Send your bank slip</h2>
      <input type="hidden" name="bookingId" value={bookingId} />
      <input type="hidden" name="token" value={token} />
      <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={`${id}-file`}>
        Photo of the slip
        <input
          id={`${id}-file`}
          name="file"
          type="file"
          accept="image/jpeg,image/png,image/webp,image/heic"
          required
          className="text-graphite file:mr-3 file:h-10 file:rounded-button file:border-0 file:bg-cloud file:px-4 file:text-graphite"
        />
      </label>
      <label className="flex flex-col gap-1.5 text-sm text-steel" htmlFor={`${id}-amount`}>
        Amount transferred ({currency})
        <input
          id={`${id}-amount`}
          name="amount"
          type="number"
          inputMode="decimal"
          min="0.01"
          step="0.01"
          required
          className="h-12 rounded-input border border-cloud bg-snow px-3 text-body text-graphite outline-none focus-visible:border-ember"
        />
      </label>
      <button
        type="submit"
        disabled={sending}
        className="h-12 rounded-button bg-obsidian font-medium text-snow shadow-button transition-opacity disabled:opacity-40"
      >
        {sending ? "Sending" : "Send slip"}
      </button>
      {state.problem && (
        <p role="alert" className="text-ember-ink text-sm">
          {state.problem.message}
        </p>
      )}
    </form>
  );
}
