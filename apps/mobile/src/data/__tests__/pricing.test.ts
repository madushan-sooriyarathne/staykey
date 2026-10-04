import { describe, expect, test } from "bun:test";
import { addDays, eachNight, inSeason, monthGrid, nightsBetween } from "../dates";
import { makeProperty } from "../defaults";
import {
  conflicts,
  nightlyRate,
  occupancy,
  quote,
  relatedUnits,
  suggestedRefund,
} from "../pricing";
import type { Booking } from "../types";

const villa = makeProperty({
  id: "p1",
  name: "Kingfisher Villa",
  slug: "kingfisher",
  bookingPageUrl: "https://kingfisher.staykey.direct",
  bookingType: "entire",
  currency: "USD",
  units: [
    { id: "u1", name: "Villa", sleeps: 6, beds: "3 bedrooms", rate: 18000, weekendRate: 22000 },
  ],
  extraGuest: { above: 4, amount: 2500 },
  seasons: [
    { id: "s1", name: "Peak", start: "12-15", end: "01-15", prices: { u1: 32000 }, minNights: 3 },
  ],
  lengthDiscounts: [{ nights: 7, percent: 10 }],
  charges: [
    { id: "c1", name: "Service charge", kind: "percent", amount: 10, per: "stay", enabled: true },
    { id: "c2", name: "Cleaning fee", kind: "fixed", amount: 3000, per: "stay", enabled: true },
    { id: "c3", name: "VAT", kind: "percent", amount: 18, per: "stay", enabled: false },
  ],
  extras: [
    {
      id: "e1",
      name: "Airport transfer",
      price: 4000,
      per: "trip",
      onRequest: false,
      enabled: true,
    },
  ],
});

const booking = (patch: Partial<Booking>): Booking => ({
  id: "b1",
  ref: "KV-2041",
  propertyId: "p1",
  unitId: "u1",
  source: "page",
  status: "confirmed",
  guest: { name: "Emma Larsen" },
  adults: 2,
  children: 0,
  checkIn: "2026-10-04",
  checkOut: "2026-10-08",
  lines: [],
  total: 83200,
  payments: [{ id: "p", amount: 83200, method: "card", at: "2026-09-28T10:00:00Z" }],
  extras: [],
  createdAt: "2026-09-28T10:00:00Z",
  ...patch,
});

describe("dates", () => {
  test("counts nights and lists each one", () => {
    expect(nightsBetween("2026-10-30", "2026-11-02")).toBe(3);
    expect(eachNight("2026-10-30", "2026-11-02")).toEqual([
      "2026-10-30",
      "2026-10-31",
      "2026-11-01",
    ]);
    expect(addDays("2026-12-31", 1)).toBe("2027-01-01");
  });

  test("seasons can wrap the new year", () => {
    expect(inSeason("2026-12-20", "12-15", "01-15")).toBe(true);
    expect(inSeason("2027-01-10", "12-15", "01-15")).toBe(true);
    expect(inSeason("2027-02-01", "12-15", "01-15")).toBe(false);
  });

  test("month grid starts on Monday", () => {
    const grid = monthGrid("2026-10-01");
    expect(grid[0]).toBe("2026-09-28");
    expect(grid.length % 7).toBe(0);
  });
});

describe("pricing", () => {
  test("weekend nights use the weekend rate and seasons win over both", () => {
    expect(nightlyRate(villa, "u1", "2026-10-07")).toBe(18000); // Wednesday
    expect(nightlyRate(villa, "u1", "2026-10-09")).toBe(22000); // Friday
    expect(nightlyRate(villa, "u1", "2026-12-20")).toBe(32000);
    expect(nightlyRate(villa, "u1", "2026-10-07", { u1: { "2026-10-07": { price: 20000 } } })).toBe(
      20000,
    );
  });

  test("a 4-night stay adds the transfer, service charge and cleaning fee", () => {
    // Sun 4 to Thu 8 Oct: four weeknights at $180.
    const q = quote(villa, {
      unitId: "u1",
      from: "2026-10-04",
      to: "2026-10-08",
      adults: 2,
      extras: ["e1"],
    });
    expect(q.nights).toBe(4);
    expect(q.lines.map((l) => [l.label, l.amount])).toEqual([
      ["4 nights", 72000],
      ["Airport transfer", 4000],
      ["Service charge 10%", 7200],
      ["Cleaning fee", 3000],
    ]);
    expect(q.total).toBe(86200);
  });

  test("long stays get the length discount and extra guests pay per night", () => {
    const q = quote(villa, { unitId: "u1", from: "2026-10-04", to: "2026-10-11", adults: 6 });
    const nights = 5 * 18000 + 2 * 22000;
    const discount = Math.round(nights * 0.1);
    const extra = 2 * 2500 * 7;
    expect(q.lines[1]?.amount).toBe(-discount);
    expect(q.lines[2]?.amount).toBe(extra);
    expect(q.total).toBe(
      nights - discount + extra + Math.round((nights - discount + extra) * 0.1) + 3000,
    );
  });
});

describe("refunds", () => {
  test("moderate refunds in full 14 days out, then half", () => {
    const b = booking({
      checkIn: "2026-11-02",
      checkOut: "2026-11-05",
      payments: [{ id: "p", amount: 66000, method: "card", at: "" }],
    });
    expect(suggestedRefund("moderate", b, "2026-10-04").amount).toBe(66000);
    expect(suggestedRefund("moderate", b, "2026-10-25").amount).toBe(33000);
    expect(suggestedRefund("strict", b, "2026-10-25").amount).toBe(0);
  });
});

describe("availability", () => {
  const house = makeProperty({
    id: "p2",
    name: "Coral Bay House",
    slug: "coralbay",
    bookingPageUrl: "",
    bookingType: "rooms",
    currency: "USD",
    units: [
      { id: "r1", name: "Ocean Room", sleeps: 2, beds: "", rate: 9000 },
      { id: "r2", name: "Garden Room", sleeps: 2, beds: "", rate: 7500 },
      {
        id: "all",
        name: "Whole house",
        sleeps: 4,
        beds: "",
        rate: 15000,
        linkedUnitIds: ["r1", "r2"],
      },
    ],
  });

  test("a whole-house unit clashes with its rooms and the reverse", () => {
    expect(relatedUnits(house, "all").sort()).toEqual(["all", "r1", "r2"]);
    expect(relatedUnits(house, "r1").sort()).toEqual(["all", "r1"]);
    const roomBooking = booking({ propertyId: "p2", unitId: "r1" });
    const clash = conflicts(house, [roomBooking], [], {
      unitId: "all",
      from: "2026-10-06",
      to: "2026-10-09",
    });
    expect(clash.bookings).toHaveLength(1);
    const free = conflicts(house, [roomBooking], [], {
      unitId: "r2",
      from: "2026-10-06",
      to: "2026-10-09",
    });
    expect(free.bookings).toHaveLength(0);
  });

  test("cancelled bookings free the dates and check-out day is open", () => {
    const b = booking({});
    expect(
      conflicts(villa, [b], [], { unitId: "u1", from: "2026-10-08", to: "2026-10-10" }).bookings,
    ).toHaveLength(0);
    expect(
      conflicts(villa, [{ ...b, status: "cancelled" }], [], {
        unitId: "u1",
        from: "2026-10-05",
        to: "2026-10-06",
      }).bookings,
    ).toHaveLength(0);
  });

  test("occupancy counts linked units as every room they cover", () => {
    const whole = booking({
      propertyId: "p2",
      unitId: "all",
      checkIn: "2026-10-01",
      checkOut: "2026-10-03",
    });
    // 2 nights x 2 rooms booked out of 2 rooms x 10 nights.
    expect(occupancy(house, [whole], "2026-10-01", "2026-10-11")).toBeCloseTo(0.2);
  });
});
