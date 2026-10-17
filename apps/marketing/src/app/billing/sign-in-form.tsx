"use client";

import { useActionState } from "react";
import { type SignInState, signIn } from "./actions";

const field =
  "w-full rounded-button border border-mist bg-snow px-4 py-3 text-obsidian outline-none focus:border-obsidian";
const button =
  "w-full rounded-button bg-obsidian px-5 py-4 text-snow shadow-button disabled:opacity-60";

/** Phone, then the code we text to it. */
export function SignInForm() {
  const [state, action, pending] = useActionState<SignInState, FormData>(signIn, {});

  return (
    <form action={action} className="flex flex-col gap-4">
      {state.phone ? (
        <>
          <input type="hidden" name="phone" value={state.phone} />
          <label className="flex flex-col gap-2">
            <span className="text-steel">Enter the code we texted to {state.phone}</span>
            <input
              name="code"
              inputMode="numeric"
              autoComplete="one-time-code"
              pattern="[0-9]{6}"
              maxLength={6}
              required
              className={field}
            />
          </label>
          {state.devCode && (
            <p className="text-caption text-steel">Development code: {state.devCode}</p>
          )}
        </>
      ) : (
        <label className="flex flex-col gap-2">
          <span className="text-steel">The phone number you sign in to StayKey with</span>
          <input
            name="phone"
            type="tel"
            autoComplete="tel"
            defaultValue="+94 "
            required
            className={field}
          />
        </label>
      )}
      {state.message && (
        <p className="text-ember-ink" aria-live="polite">
          {state.message}
        </p>
      )}
      <button type="submit" disabled={pending} className={button}>
        {state.phone ? "Sign in" : "Text me a code"}
      </button>
    </form>
  );
}
