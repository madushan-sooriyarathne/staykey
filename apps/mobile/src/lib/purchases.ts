import { Platform } from "react-native";
import Purchases from "react-native-purchases";

export type PlanId = "villa" | "guesthouse" | "collection";
export type Period = "monthly" | "yearly";

export type Plan = {
  id: PlanId;
  name: string;
  /** Upper bound of bookable units on this plan. */
  units: number;
  unitsLabel: string;
  /** List prices in whole US dollars. The store sheet shows the local price. */
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

/** The store product for a plan and period, as set up in App Store Connect and Google Play. */
export function productId(plan: PlanId, period: Period): string {
  return `staykey_${plan}_${period}`;
}

type PurchaseResult = { ok: true } | { ok: false; cancelled?: boolean; message: string };

const UNAVAILABLE: PurchaseResult = {
  ok: false,
  message: "Purchases aren't available in this build.",
};

const apiKey = Platform.select({
  ios: process.env.EXPO_PUBLIC_REVENUECAT_APPLE_KEY,
  android: process.env.EXPO_PUBLIC_REVENUECAT_GOOGLE_KEY,
});

let configured = false;

/**
 * Sets RevenueCat up on first use. Web builds and builds without a key have no store, so
 * purchases report that instead.
 */
function ready(): boolean {
  if (!apiKey) return false;
  if (!configured) {
    Purchases.configure({ apiKey });
    configured = true;
  }
  return true;
}

function cancelled(e: unknown): boolean {
  return typeof e === "object" && e !== null && "userCancelled" in e && e.userCancelled === true;
}

function messageOf(e: unknown): string {
  return e instanceof Error ? e.message : "The store couldn't finish the purchase. Try again.";
}

/**
 * Makes the account the RevenueCat customer, so a purchase belongs to the account rather than
 * the person or the phone. The API reads plans back by account id.
 */
export async function identify(accountId: string): Promise<void> {
  if (!accountId || !ready()) return;
  try {
    await Purchases.logIn(accountId);
  } catch (e) {
    // A purchase identifies the account again first, so this only delays it.
    console.warn("RevenueCat logIn failed", e);
  }
}

/**
 * Buys the plan through the store. Access follows from the API, which hears from RevenueCat;
 * callers refresh the subscription afterwards rather than trusting the store's answer.
 */
export async function purchase(
  accountId: string,
  plan: PlanId,
  period: Period,
): Promise<PurchaseResult> {
  if (!ready()) return UNAVAILABLE;
  try {
    await Purchases.logIn(accountId);
    const [product] = await Purchases.getProducts([productId(plan, period)]);
    if (!product) return { ok: false, message: "This plan isn't available in the store yet." };
    await Purchases.purchaseStoreProduct(product);
    return { ok: true };
  } catch (e) {
    if (cancelled(e)) return { ok: false, cancelled: true, message: "" };
    return { ok: false, message: messageOf(e) };
  }
}

/** Restores the store's purchases to the account. */
export async function restore(accountId: string): Promise<PurchaseResult> {
  if (!ready()) return UNAVAILABLE;
  try {
    await Purchases.logIn(accountId);
    await Purchases.restorePurchases();
    return { ok: true };
  } catch (e) {
    return { ok: false, message: messageOf(e) };
  }
}
