import createClient, { type ClientOptions } from "openapi-fetch";
import type { components, paths } from "./schema";

export type { components, paths } from "./schema";

type Schemas = components["schemas"];
export type Property = Schemas["Property"];
export type NewProperty = Schemas["NewProperty"];
export type PublicProperty = Schemas["PublicProperty"];
export type BookingType = Schemas["BookingType"];
export type Currency = Schemas["Currency"];
export type ApiError = Schemas["Error"];
export type Health = Schemas["Health"];

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
