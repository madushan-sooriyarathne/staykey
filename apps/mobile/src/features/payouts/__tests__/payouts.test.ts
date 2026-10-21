import { describe, expect, test } from "bun:test";
import { isValidAccountNumber, isValidNIC } from "../payouts";

describe("isValidNIC", () => {
  test("takes the old format with V", () => {
    expect(isValidNIC("851234567v")).toBe(true);
  });
  test("takes the new 12 digit format", () => {
    expect(isValidNIC("1985 1234 5678")).toBe(true);
  });
  test("refuses a short number", () => {
    expect(isValidNIC("85123456V")).toBe(false);
  });
});

describe("isValidAccountNumber", () => {
  test("takes digits with spaces and dashes", () => {
    expect(isValidAccountNumber("8001 2345-67")).toBe(true);
  });
  test("refuses letters", () => {
    expect(isValidAccountNumber("12AB5678")).toBe(false);
  });
});
