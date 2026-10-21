import { describe, expect, test } from "bun:test";
import { cleanBreadcrumb, cleanEvent, cleanUrl, reportingEnabled, scrub } from "../index";

describe("reportingEnabled", () => {
  test("production with a DSN reports", () => {
    expect(reportingEnabled("production", "https://key@o1.ingest.sentry.io/1")).toBe(true);
  });
  test("development never reports", () => {
    expect(reportingEnabled("development", "https://key@o1.ingest.sentry.io/1")).toBe(false);
  });
  test("staging never reports", () => {
    expect(reportingEnabled("staging", "https://key@o1.ingest.sentry.io/1")).toBe(false);
  });
  test("production without a DSN doesn't report", () => {
    expect(reportingEnabled("production", "")).toBe(false);
  });
});

describe("scrub", () => {
  const cases: [string, string][] = [
    ["a Sri Lankan mobile number", "guest +94 77 123 4567 booked"],
    ["a local mobile number", "call 0771234567"],
    ["an email address", "to nadeesha.perera@example.com"],
    ["a live key", "key sk_live_abc123 refused"],
    ["a test key", "key sk_test_owner_fixture-01 refused"],
    ["a bearer token", "Authorization: Bearer abc.def"],
    ["an old NIC", "nic 851234567V"],
    ["a new NIC", "nic 198512345678"],
  ];
  for (const [name, input] of cases) {
    test(`removes ${name}`, () => {
      expect(scrub(input)).toContain("[removed]");
    });
  }
  test("keeps ids and amounts", () => {
    const s = "booking 01a15d07-203b-76d3-9add-9a39bf9ee8db charged 5418000";
    expect(scrub(s)).toBe(s);
  });
});

describe("cleanUrl", () => {
  test("drops the stay page's token", () => {
    expect(cleanUrl("https://kingfisher.staykey.direct/stay/01a1?t=secret&paid=1")).toBe(
      "https://kingfisher.staykey.direct/stay/01a1",
    );
  });
  test("replaces an invite token", () => {
    expect(cleanUrl("https://staykey.direct/join/W3_NICS5")).toBe(
      "https://staykey.direct/join/:token",
    );
  });
  test("replaces an email confirmation token", () => {
    expect(cleanUrl("/verify-email/abc123")).toBe("/verify-email/:token");
  });
  test("replaces the app's invite link query", () => {
    expect(cleanUrl("staykey://join?token=abc")).toBe("staykey://join");
  });
});

describe("cleanEvent", () => {
  test("keeps only the user's id", () => {
    const e = cleanEvent({ user: { id: "u1", email: "emma@example.com", ip_address: "1.2.3.4" } });
    expect(e.user).toEqual({ id: "u1" });
  });
  test("keeps only the method, cleaned URL and user agent of a request", () => {
    const e = cleanEvent({
      request: {
        method: "POST",
        url: "https://x.staykey.direct/stay/1?t=tok",
        headers: { "User-Agent": "Safari", Cookie: "a=b", Authorization: "Bearer x" },
        data: { name: "Emma" },
        query_string: "t=tok",
      },
    });
    expect(e.request).toEqual({
      method: "POST",
      url: "https://x.staykey.direct/stay/1",
      headers: { "User-Agent": "Safari" },
    });
  });
  test("scrubs exception values", () => {
    const e = cleanEvent({ exception: { values: [{ value: "no guest +94771234567" }] } });
    expect(e.exception?.values?.[0]?.value).toBe("no guest [removed]");
  });
  test("drops the query from a request path in contexts", () => {
    const e = cleanEvent({ contexts: { nextjs: { request_path: "/stay/1?t=guest-token" } } });
    expect(e.contexts).toEqual({ nextjs: { request_path: "/stay/1" } });
  });
  test("drops the query from an app link in contexts", () => {
    const e = cleanEvent({ contexts: { route: { url: "staykey://join?token=invite-123" } } });
    expect(e.contexts).toEqual({ route: { url: "staykey://join" } });
  });
  test("drops the phone's name", () => {
    const e = cleanEvent({ contexts: { device: { name: "Nimal's iPhone", model: "iPhone16,1" } } });
    expect(e.contexts).toEqual({ device: { model: "iPhone16,1" } });
  });
  test("scrubs other strings in contexts", () => {
    const e = cleanEvent({ contexts: { app: { note: "for emma@example.com" } } });
    expect(e.contexts).toEqual({ app: { note: "for [removed]" } });
  });
});

describe("cleanBreadcrumb", () => {
  test("drops bodies and cleans navigation URLs", () => {
    const b = cleanBreadcrumb({ data: { to: "/stay/1?t=tok", body: '{"phone":"0771234567"}' } });
    expect(b.data).toEqual({ to: "/stay/1" });
  });
});
