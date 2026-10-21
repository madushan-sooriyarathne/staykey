import { describe, expect, test } from "bun:test";
import type { Plan } from "@staykey/api-client";
import { parsePeriod, unitsLabel, yearlySaving } from "../plans";

const villa: Plan = {
  id: "villa",
  name: "Villa",
  units: 1,
  currency: "LKR",
  monthly: 290_000,
  yearly: 2_900_000,
};

describe("yearlySaving", () => {
  test("ten monthly prices a year is two months free", () => {
    expect(yearlySaving(villa)).toBe("2 months free");
  });
  test("no saving says nothing", () => {
    expect(yearlySaving({ ...villa, yearly: 3_480_000 })).toBe("");
  });
});

describe("unitsLabel", () => {
  test("a one unit plan is for one villa or room", () => {
    expect(unitsLabel(villa)).toBe("1 villa or room");
  });
  test("a larger plan counts rooms", () => {
    expect(unitsLabel({ ...villa, units: 6 })).toBe("Up to 6 rooms");
  });
});

describe("parsePeriod", () => {
  test("yearly is read", () => {
    expect(parsePeriod("yearly")).toBe("yearly");
  });
  test("anything else is monthly", () => {
    expect(parsePeriod(["yearly", "x"])).toBe("monthly");
  });
});
