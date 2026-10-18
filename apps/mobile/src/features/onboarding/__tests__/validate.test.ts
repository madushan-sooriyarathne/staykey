import { describe, expect, test } from "bun:test";
import { isOptionalEmail } from "../validate";

describe("isOptionalEmail", () => {
  test("accepts no email", () => {
    expect(isOptionalEmail("  ")).toBe(true);
  });

  test("accepts an address", () => {
    expect(isOptionalEmail("nimal@example.com")).toBe(true);
  });

  test("refuses text without a domain", () => {
    expect(isOptionalEmail("nimal@")).toBe(false);
  });
});
