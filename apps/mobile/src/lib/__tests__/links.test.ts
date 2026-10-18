import { describe, expect, test } from "bun:test";
import { joinPath } from "../links";

describe("joinPath", () => {
  test("turns a universal invite link into the join screen with its token", () => {
    expect(joinPath("https://staykey.direct/join/abc_DEF-123")).toBe("/join?token=abc_DEF-123");
  });

  test("reads the path the router passes without a host", () => {
    expect(joinPath("/join/abc123")).toBe("/join?token=abc123");
  });

  test("leaves other paths alone", () => {
    expect(joinPath("/booking/42")).toBe("/booking/42");
  });
});
