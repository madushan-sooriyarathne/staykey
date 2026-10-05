import { describe, expect, test } from "bun:test";
import type { Booking as ApiBooking } from "@staykey/api-client";
import { toBooking, toCalendar } from "../from-api";

const stay: ApiBooking = {
  id: "b1",
  ref: "KV-2041",
  propertyId: "p1",
  unitId: "u1",
  source: "whatsapp",
  status: "cancelled",
  guest: { name: "Nimali Perera" },
  adults: 2,
  children: 0,
  checkIn: "2026-10-05",
  checkOut: "2026-10-07",
  currency: "USD",
  total: 36000,
  lines: [{ label: "2 nights", amount: 36000 }],
  extras: [],
  payments: [
    {
      id: "pay1",
      kind: "payment",
      method: "cash",
      amount: 10000,
      note: "",
      receivedAt: "2026-10-01T10:00:00Z",
    },
    {
      id: "pay2",
      kind: "refund",
      method: "cash",
      amount: 4000,
      note: "",
      receivedAt: "2026-10-02T10:00:00Z",
    },
  ],
  slips: [
    {
      id: "s1",
      amount: 26000,
      status: "rejected",
      uploadedAt: "2026-10-01T09:00:00Z",
      url: "https://m/1.jpg",
    },
    {
      id: "s2",
      amount: 26000,
      status: "pending",
      uploadedAt: "2026-10-02T09:00:00Z",
      url: "https://m/2.jpg",
    },
  ],
  guestNote: "",
  ownerNote: "",
  cancel: { reason: "Plans changed", at: "2026-10-02T10:00:00Z" },
  version: 3,
  createdAt: "2026-09-30T10:00:00Z",
  updatedAt: "2026-10-02T10:00:00Z",
};

describe("toBooking", () => {
  test("a refund payment reads as the refund method", () => {
    expect(toBooking(stay).payments.map((p) => p.method)).toEqual(["cash", "refund"]);
  });

  test("the pending slip is the one shown", () => {
    expect(toBooking(stay).slip?.id).toBe("s2");
  });

  test("the cancellation carries what was refunded", () => {
    expect(toBooking(stay).cancel?.refund).toBe(4000);
  });

  test("a caretaker's copy without money reads as zero", () => {
    const { total: _total, lines: _lines, payments: _payments, slips: _slips, ...bare } = stay;
    expect(toBooking(bare).total).toBe(0);
  });
});

describe("toCalendar", () => {
  test("overrides index by unit, then night", () => {
    const { overrides } = toCalendar({
      blocks: [],
      rateOverrides: [{ unitId: "u1", night: "2026-10-05", price: 9900, closedToArrival: false }],
    });
    expect(overrides).toEqual({
      u1: { "2026-10-05": { price: 9900, minNights: undefined, closedToArrival: undefined } },
    });
  });
});
