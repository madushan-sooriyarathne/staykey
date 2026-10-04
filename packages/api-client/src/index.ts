import createClient, { type ClientOptions } from "openapi-fetch";
import type { components, paths } from "./schema";

export type { Middleware } from "openapi-fetch";
export type { components, paths } from "./schema";

type Schemas = components["schemas"];
export type Property = Schemas["Property"];
export type NewProperty = Schemas["NewProperty"];
export type PublicProperty = Schemas["PublicProperty"];
export type BookingType = Schemas["BookingType"];
export type Currency = Schemas["Currency"];
export type ApiError = Schemas["Error"];
export type Health = Schemas["Health"];
export type User = Schemas["User"];
export type UserPatch = Schemas["UserPatch"];
export type Account = Schemas["Account"];
export type Role = Schemas["Role"];
export type Me = Schemas["Me"];
export type OtpSent = Schemas["OtpSent"];
export type SignIn = Schemas["SignIn"];
export type AuthTokens = Schemas["AuthTokens"];

/** Header that names the account an owner route acts on. */
export const ACCOUNT_HEADER = "X-Account-Id";

export const PRODUCTION_API_URL = "https://api.staykey.direct";

/** Typed client for the StayKey API, generated from packages/api-spec/openapi.yaml. */
export function createStayKeyClient(baseUrl: string = PRODUCTION_API_URL, options?: ClientOptions) {
  return createClient<paths>({ ...options, baseUrl });
}

export type StayKeyClient = ReturnType<typeof createStayKeyClient>;

/** Formats an amount in minor units, for example 18000 USD as "$180". */
export function formatMoney(amountMinor: number, currency: Currency, locale = "en-US"): string {
  const amount = amountMinor / 100;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    maximumFractionDigits: Number.isInteger(amount) ? 0 : 2,
  }).format(amount);
}

/**
 * Suggests a booking page slug from a property name, for example "Kingfisher Villa" becomes
 * "kingfisher-villa". Mirrors domain.Slugify in apps/api so suggestions match what the API accepts.
 */
export function suggestSlug(name: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug.slice(0, 40).replace(/-+$/, "");
}

export const SLUG_PATTERN = /^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$/;
