import { describe, expect, test } from "bun:test";
import { toRupees } from "@staykey/api-client";
import { toPatch } from "../from-api";
import { paidOf } from "../pricing";
import type { Booking, PropertyConfig } from "../types";

const payments: PropertyConfig["payments"] = {
  bank: {
    enabled: false,
    bankName: "",
    accountName: "",
    accountNumber: "",
    payWithinHours: 24,
    cancelIfUnpaid: true,
  },
  atProperty: false,
  cards: "pending",
  cardCharge: "full",
  cardRate: 30000,
  cardsMissing: ["connection"],
};

describe("card payments", () => {
  test("cards waiting for a connection are saved as switched on", () => {
    expect(toPatch({ payments }).payments?.cards).toBe("on");
  });

  test("what keeps cards off is never sent back", () => {
    expect(toPatch({ payments }).payments).not.toHaveProperty("cardsMissing");
  });

  test("a failed card refund doesn't count against what was paid", () => {
    const b: Pick<Booking, "payments"> = {
      payments: [
        { id: "1", amount: 36000, method: "card", at: "2026-10-01T10:00:00Z", status: "succeeded" },
        {
          id: "2",
          amount: 18000,
          method: "refund",
          via: "card",
          at: "2026-10-02T10:00:00Z",
          status: "failed",
        },
      ],
    };
    expect(paidOf(b)).toBe(36000);
  });

  test("rupees round up like the API", () => {
    expect(toRupees(12345, 29876)).toBe(3688193);
  });
});
