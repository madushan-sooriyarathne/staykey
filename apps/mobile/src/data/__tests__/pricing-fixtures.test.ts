import { describe, expect, test } from "bun:test";
import fixtures from "@staykey/api-spec/fixtures/pricing.json";
import { makeProperty } from "../defaults";
import { quote } from "../pricing";
import type { Charge, Extra, Promo } from "../types";

// The API prices stays in Go from the same cases (apps/api/internal/domain/pricing), so a
// preview in the app matches the total the server stores.
const properties = fixtures.properties as Record<string, (typeof fixtures.properties)["villa"]>;

describe("quote matches the shared pricing fixtures", () => {
  for (const c of fixtures.cases) {
    test(c.name, () => {
      const fp = properties[c.property];
      if (!fp) throw new Error(`fixture property ${c.property} is missing`);
      const p = makeProperty({
        id: c.property,
        name: c.property,
        slug: c.property,
        bookingPageUrl: "",
        bookingType: "entire",
        currency: "USD",
        units: fp.units.map((u) => ({ name: "Unit", sleeps: 2, beds: "", ...u })),
        extraGuest: fp.extraGuest,
        seasons: fp.seasons.map((s, i) => ({ id: `s${i}`, name: "Season", ...s })),
        lengthDiscounts: fp.lengthDiscounts,
        promos: fp.promos.map((x) => ({ ...x, kind: x.kind as Promo["kind"], used: 0 })),
        extras: fp.extras.map((e) => ({
          ...e,
          per: e.per as Extra["per"],
          onRequest: false,
          enabled: true,
        })),
        charges: fp.charges.map((x, i) => ({
          ...x,
          id: `c${i}`,
          kind: x.kind as Charge["kind"],
          per: x.per as Charge["per"],
        })),
      });

      const q = quote(p, c.input);

      expect({ nights: q.nights, lines: q.lines, total: q.total }).toEqual(c.expect);
    });
  }
});
