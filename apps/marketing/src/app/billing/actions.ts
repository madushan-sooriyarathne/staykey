"use server";

import type { BillingPeriod, PlanId } from "@staykey/api-client";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { checkout, sendCode, signOut, verifyCode } from "@/lib/api";
import { ACCESS_COOKIE, ACCOUNT_COOKIE, cookieOptions, REFRESH_COOKIE } from "@/lib/session";

export type SignInState = {
  /** Set once a code was sent, which moves the form to the code step. */
  phone?: string;
  message?: string;
  /** The code itself, from development servers only. */
  devCode?: string;
};

/** Spaces and dashes are fine; a local 07 number is read as Sri Lankan. */
function normalizePhone(raw: string): string {
  const digits = raw.replace(/[\s-]/g, "");
  return digits.startsWith("0") ? `+94${digits.slice(1)}` : digits;
}

/** Sends a code to the phone, then signs in with it. */
export async function signIn(_prev: SignInState, form: FormData): Promise<SignInState> {
  const phone = normalizePhone(String(form.get("phone") ?? ""));
  const code = String(form.get("code") ?? "").trim();

  if (!code) {
    const sent = await sendCode(phone);
    if (!sent.ok) return { message: sent.message };
    return {
      phone: sent.data.phone,
      devCode: process.env.NODE_ENV === "production" ? undefined : sent.data.devCode,
    };
  }

  const signedIn = await verifyCode(phone, code);
  if (!signedIn.ok) return { phone, message: signedIn.message };
  const { tokens } = signedIn.data;
  const jar = await cookies();
  jar.set(ACCESS_COOKIE, tokens.accessToken, cookieOptions(new Date(tokens.accessTokenExpiresAt)));
  jar.set(
    REFRESH_COOKIE,
    tokens.refreshToken,
    cookieOptions(new Date(tokens.refreshTokenExpiresAt)),
  );
  redirect("/billing");
}

export type CheckoutState = { message?: string };

const PLANS: PlanId[] = ["villa", "guesthouse", "collection"];
const PERIODS: BillingPeriod[] = ["monthly", "yearly"];

/** Starts a Payments.lk checkout and sends the owner to it. */
export async function startCheckout(_prev: CheckoutState, form: FormData): Promise<CheckoutState> {
  const jar = await cookies();
  const token = jar.get(ACCESS_COOKIE)?.value;
  if (!token) redirect("/billing");

  const account = String(form.get("account") ?? "");
  const plan = PLANS.find((p) => p === form.get("plan"));
  const period = PERIODS.find((p) => p === form.get("period"));
  if (!account || !plan || !period) return { message: "Choose a plan and how often to pay." };

  const started = await checkout(token, account, plan, period);
  if (!started.ok) return { message: started.message };
  jar.set(ACCOUNT_COOKIE, account, cookieOptions());
  redirect(started.data.url);
}

export async function signOutOfBilling(): Promise<void> {
  const jar = await cookies();
  const refresh = jar.get(REFRESH_COOKIE)?.value;
  if (refresh) await signOut(refresh);
  for (const name of [ACCESS_COOKIE, REFRESH_COOKIE, ACCOUNT_COOKIE]) {
    jar.delete({ name, path: "/billing" });
  }
  redirect("/billing");
}
