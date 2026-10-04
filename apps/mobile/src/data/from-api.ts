import type { Property } from "@staykey/api-client";
import { makeProperty } from "./defaults";
import type { Currency, PropertyConfig } from "./types";

/**
 * Turns a property from the API into the on-device model the screens read. The API only knows a
 * property's basics and base rate until phase 2, so it gets one unit at that rate; extra detail
 * passed in (rooms, rates, location) wins.
 */
export function localProperty(
  p: Property,
  extra: Partial<Omit<PropertyConfig, "id" | "slug" | "bookingPageUrl">> = {},
): PropertyConfig {
  return makeProperty({
    id: p.id,
    name: p.name,
    slug: p.slug,
    bookingPageUrl: p.bookingPageUrl,
    bookingType: p.bookingType,
    currency: p.currency as Currency,
    location: p.location ?? "",
    units: [
      {
        id: `${p.id}_u1`,
        name: p.bookingType === "entire" ? p.name : "Room 1",
        sleeps: p.bookingType === "entire" ? 4 : 2,
        beds: "",
        rate: p.baseRate,
      },
    ],
    ...extra,
  });
}
