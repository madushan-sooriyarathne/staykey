import { describe, expect, test } from "bun:test";
import type { Property } from "@staykey/api-client";
import { toNewProperty, toPatch, toPropertyConfig } from "../from-api";

const api: Property = {
  id: "p1",
  slug: "coralbay",
  name: "Coral Bay House",
  bookingType: "rooms",
  location: "Mirissa",
  currency: "USD",
  baseRate: 7500,
  bookingPageUrl: "https://coralbay.staykey.direct",
  createdAt: "2026-10-01T00:00:00Z",
  description: "",
  amenities: ["Pool"],
  checkIn: "14:00",
  checkOut: "11:00",
  timeZone: "Asia/Colombo",
  photos: [{ id: "ph1", key: "accounts/a/photos/1.jpg", url: "https://m/1.jpg", caption: "" }],
  units: [
    {
      id: "u1",
      name: "Ocean Room",
      sleeps: 2,
      beds: "",
      rate: 9000,
      weekendRate: 10000,
      linkedUnitIds: [],
    },
    { id: "u2", name: "Whole house", sleeps: 4, beds: "", rate: 15000, linkedUnitIds: ["u1"] },
  ],
  extraGuest: { above: 0, amount: 0 },
  seasons: [{ id: "s1", name: "Peak", start: "12-15", end: "01-15", prices: { u1: 15000 } }],
  lengthDiscounts: [{ nights: 7, percent: 10 }],
  rules: { minNights: 1, maxNights: 30, sameDayCutoff: null, windowMonths: 12, closedArrival: [] },
  policy: "flexible",
  depositPercent: 30,
  balanceDueDays: 14,
  houseRules: [],
  charges: [{ id: "c1", name: "VAT", kind: "percent", amount: 18, per: "stay", enabled: false }],
  extras: [],
  promos: [{ id: "pr1", code: "RETURN10", kind: "percent", amount: 10, used: 4, limit: 20 }],
  payments: {
    bank: {
      enabled: true,
      bankName: "Sampath Bank",
      accountName: "N Perera",
      accountNumber: "001234567890",
      payWithinHours: 24,
      cancelIfUnpaid: true,
    },
    atProperty: true,
    cards: "off",
  },
  booking: { mode: "request", replyHours: 24, holdMinutes: 15, displayCurrencies: ["USD"] },
  branding: { color: "#09090b", logoKey: "accounts/a/logos/1.png", logoUrl: "https://m/l.png" },
  icalFeeds: [{ id: "f1", channel: "airbnb", url: "", status: "pending", upcoming: 0 }],
  icalExportToken: "abc",
};

describe("from-api", () => {
  test("maps the API property to the screens' model", () => {
    const p = toPropertyConfig(api);
    expect(p.extraGuest).toBeNull();
    expect(p.units[0]?.linkedUnitIds).toBeUndefined();
    expect(p.units[1]?.linkedUnitIds).toEqual(["u1"]);
    expect(p.photos[0]).toEqual({
      uri: "https://m/1.jpg",
      key: "accounts/a/photos/1.jpg",
      caption: undefined,
    });
    expect(p.branding).toEqual({
      color: "#09090b",
      logoUri: "https://m/l.png",
      logoKey: "accounts/a/logos/1.png",
    });
    expect(p.ical[0]?.status).toBe("pending");
    expect(p.promos[0]?.used).toBe(4);
  });

  test("sends only what changed, with ids, and never the used count", () => {
    const p = toPropertyConfig(api);
    const patch = toPatch({
      promos: p.promos,
      extraGuest: null,
      photos: [...p.photos, { uri: "file://new.jpg" }],
    });
    expect(Object.keys(patch).sort()).toEqual(["extraGuest", "photos", "promos"]);
    expect(patch.promos?.[0]).not.toHaveProperty("used");
    expect(patch.promos?.[0]?.id).toBe("pr1");
    expect(patch.extraGuest).toEqual({ above: 0, amount: 0 });
    // Photos still on the device (no key yet) are left out.
    expect(patch.photos).toEqual([{ key: "accounts/a/photos/1.jpg", caption: undefined }]);
  });

  test("a new property carries its basics at the top level and the rest as setup", () => {
    const body = toNewProperty(toPropertyConfig(api));
    expect(body.name).toBe("Coral Bay House");
    expect(body.bookingType).toBe("rooms");
    expect(body.setup).not.toHaveProperty("name");
    expect(body.setup).not.toHaveProperty("location");
    expect(body.setup?.units?.[1]?.linkedUnitIds).toEqual(["u1"]);
    expect(body.setup?.seasons?.[0]?.prices).toEqual({ u1: 15000 });
  });
});
