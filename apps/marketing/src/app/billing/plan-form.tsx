"use client";

import type { BillingPeriod, PlanId } from "@staykey/api-client";
import { useActionState, useState } from "react";
import { type CheckoutState, startCheckout } from "./actions";

export type PlanChoice = {
  id: PlanId;
  name: string;
  units: string;
  /** Prices already written out on the server, so both renders match. */
  monthly: string;
  yearly: string;
};

type Props = {
  account: string;
  plans: PlanChoice[];
  /** The plan that covers the account's properties, chosen to start with. */
  fits: PlanId;
  submitLabel: string;
};

/** Plan and period, then off to Payments.lk. */
export function PlanForm({ account, plans, fits, submitLabel }: Props) {
  const [state, action, pending] = useActionState<CheckoutState, FormData>(startCheckout, {});
  const [period, setPeriod] = useState<BillingPeriod>("yearly");

  return (
    <form action={action} className="flex flex-col gap-5">
      <input type="hidden" name="account" value={account} />
      <fieldset className="flex gap-2">
        <legend className="sr-only">How often to pay</legend>
        {(["monthly", "yearly"] as const).map((p) => (
          <label
            key={p}
            className="flex-1 cursor-pointer rounded-button border border-mist px-4 py-3 text-center has-checked:border-obsidian has-checked:bg-snow"
          >
            <input
              type="radio"
              name="period"
              value={p}
              checked={period === p}
              onChange={() => setPeriod(p)}
              className="sr-only"
            />
            {p === "monthly" ? "1 month" : "1 year"}
          </label>
        ))}
      </fieldset>
      <fieldset className="flex flex-col gap-2">
        <legend className="sr-only">Plan</legend>
        {plans.map((plan) => (
          <label
            key={plan.id}
            className="flex cursor-pointer items-center justify-between gap-4 rounded-button border border-mist bg-paper px-4 py-4 has-checked:border-obsidian has-checked:bg-snow"
          >
            <span className="flex items-center gap-3">
              <input type="radio" name="plan" value={plan.id} defaultChecked={plan.id === fits} />
              <span className="flex flex-col">
                <span className="font-semibold text-obsidian">{plan.name}</span>
                <span className="text-caption text-steel">{plan.units}</span>
              </span>
            </span>
            <span className="text-obsidian">
              {period === "monthly" ? plan.monthly : plan.yearly}
            </span>
          </label>
        ))}
      </fieldset>
      {state.message && (
        <p className="text-ember-ink" aria-live="polite">
          {state.message}
        </p>
      )}
      <button
        type="submit"
        disabled={pending}
        className="w-full rounded-button bg-obsidian px-5 py-4 text-snow shadow-button disabled:opacity-60"
      >
        {submitLabel}
      </button>
      <p className="text-caption text-steel">
        You pay once for the period you choose, by card through Payments.lk. Nothing renews on its
        own; come back here to pay for the next one.
      </p>
    </form>
  );
}
