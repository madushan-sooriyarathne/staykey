import { Platform } from "react-native";

export type PlanId = "villa" | "guesthouse" | "collection";
export type Period = "monthly" | "yearly";

export type Plan = {
  id: PlanId;
  name: string;
  /** Upper bound of bookable units on this plan. */
  units: number;
  unitsLabel: string;
  /** Prices in whole US dollars. App Store and Google Play show local prices once wired up. */
  monthly: number;
  yearly: number;
};

export const PLANS: Plan[] = [
  { id: "villa", name: "Villa", units: 1, unitsLabel: "1 villa or room", monthly: 9, yearly: 90 },
  {
    id: "guesthouse",
    name: "Guesthouse",
    units: 6,
    unitsLabel: "Up to 6 rooms",
    monthly: 19,
    yearly: 190,
  },
  {
    id: "collection",
    name: "Collection",
    units: 20,
    unitsLabel: "Up to 20 rooms",
    monthly: 39,
    yearly: 390,
  },
];

/** Every plan includes every feature. Plans only differ by how many units can be booked. */
export const INCLUDED = [
  "Booking page, widget and iFrame",
  "Calendar sync with Airbnb and Booking.com",
  "No commission on any booking",
  "Team access for managers and caretakers",
  "WhatsApp and email guest messages",
];

export function planFor(units: number): Plan {
  return PLANS.find((p) => units <= p.units) ?? (PLANS[PLANS.length - 1] as Plan);
}

export function priceLabel(plan: Plan, period: Period): string {
  return period === "monthly" ? `$${plan.monthly}/month` : `$${plan.yearly}/year`;
}

export const STORE_NAME = Platform.OS === "android" ? "Google Play" : "the App Store";

type PurchaseResult = { ok: true } | { ok: false; cancelled?: boolean; message: string };

/**
 * Stand-in for the in-app purchase flow.
 * TODO: replace with RevenueCat (react-native-purchases) offerings and purchasePackage, and
 * mirror entitlement changes to the API so the booking page knows the subscription state.
 */
export async function purchase(_plan: PlanId, _period: Period): Promise<PurchaseResult> {
  await new Promise((r) => setTimeout(r, 900));
  return { ok: true };
}

/** TODO: RevenueCat restorePurchases. */
export async function restore(): Promise<{ restored: boolean }> {
  await new Promise((r) => setTimeout(r, 700));
  return { restored: false };
}
