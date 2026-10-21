import { describe, expect, test } from "bun:test";
import { storeFor } from "../store-for";

describe("storeFor", () => {
  test("an iPhone goes to the App Store", () => {
    expect(
      storeFor("Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15"),
    ).toBe("ios");
  });
  test("an older iPad goes to the App Store", () => {
    expect(storeFor("Mozilla/5.0 (iPad; CPU OS 12_0 like Mac OS X)")).toBe("ios");
  });
  test("an Android phone goes to Google Play", () => {
    expect(
      storeFor("Mozilla/5.0 (Linux; Android 15; Pixel 9) AppleWebKit/537.36 Chrome/130 Mobile"),
    ).toBe("android");
  });
  test("a desktop browser gets the download section", () => {
    expect(
      storeFor("Mozilla/5.0 (Macintosh; Intel Mac OS X 15_0) AppleWebKit/605.1.15 Safari/605.1.15"),
    ).toBeNull();
  });
  test("no user agent gets the download section", () => {
    expect(storeFor(null)).toBeNull();
  });
});
