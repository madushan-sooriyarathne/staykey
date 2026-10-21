import type { BillingPeriod, Plan } from "@staykey/api-client";

/** Everything every plan includes. Plans only differ by how many units can be booked. Kept in
 * step by hand with INCLUDED in apps/mobile/src/lib/purchases.ts. */
export const INCLUDED = [
  "Booking page, widget and iFrame",
  "Calendar sync with Airbnb and Booking.com",
  "No commission on any booking",
  "Team access for managers and caretakers",
  "WhatsApp and email guest messages",
] as const;

/** How many units a plan is for, as owners say it. */
export function unitsLabel(plan: Plan): string {
  return plan.units === 1 ? "1 villa or room" : `Up to ${plan.units} rooms`;
}

/**
 * What paying yearly saves, in whole months of the monthly price: "2 months free". Empty when
 * yearly saves less than a month.
 */
export function yearlySaving(plan: Plan): string {
  if (plan.monthly <= 0) return "";
  const months = Math.floor((plan.monthly * 12 - plan.yearly) / plan.monthly);
  if (months < 1) return "";
  return months === 1 ? "1 month free" : `${months} months free`;
}

export function priceFor(plan: Plan, period: BillingPeriod): number {
  return period === "yearly" ? plan.yearly : plan.monthly;
}

export function parsePeriod(value: string | string[] | undefined): BillingPeriod {
  return value === "yearly" ? "yearly" : "monthly";
}
