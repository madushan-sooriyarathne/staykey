import { describe, expect, test } from "bun:test";

import { ApiError, isReportable, OfflineError, unwrap } from "../errors";

describe("isReportable", () => {
  test("reports a server error", () => {
    expect(isReportable(new ApiError(502))).toBe(true);
  });
  test("leaves out a refusal", () => {
    expect(isReportable(new ApiError(409, { code: "dates_taken" }))).toBe(false);
  });
  test("leaves out being offline", () => {
    expect(isReportable(new OfflineError())).toBe(false);
  });
  test("reports a bug in the app", () => {
    expect(isReportable(new TypeError("cannot read 'nights' of undefined"))).toBe(true);
  });
});

describe("unwrap", () => {
  test("keeps the API's request id on a failed call", async () => {
    const response = new Response(null, { status: 500, headers: { "X-Request-Id": "req-1" } });
    const error = await unwrap(Promise.resolve({ response })).catch((e: ApiError) => e);
    expect(error).toMatchObject({ status: 500, requestId: "req-1" });
  });
});
