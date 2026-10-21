import { describe, expect, test } from "bun:test";
import { formatRupees } from "../format";

describe("formatRupees", () => {
  test("rupees are written with Rs", () => {
    expect(formatRupees(290_000, "LKR")).toBe("Rs\u00a02,900");
  });
  test("cents show when there are any", () => {
    expect(formatRupees(5_255_460, "LKR")).toBe("Rs\u00a052,554.60");
  });
});
