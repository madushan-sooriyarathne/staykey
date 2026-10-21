import * as Sentry from "@sentry/react-native";
import { cleanBreadcrumb, cleanEvent, reportingEnabled } from "@staykey/observe";

import { ApiError, isReportable } from "@/api/errors";
import { useSession } from "@/lib/session";

const dsn = process.env.EXPO_PUBLIC_SENTRY_DSN;

/**
 * Only release builds made for production report, so development builds, Expo Go and
 * staging spend none of Sentry's free quota.
 */
export const isReporting = !__DEV__ && reportingEnabled(process.env.EXPO_PUBLIC_APP_ENV, dsn);

if (isReporting) {
  Sentry.init({
    dsn,
    environment: "production",
    // The release is left to the SDK, which names it after the native build so it matches the
    // source maps the build uploaded.
    sendDefaultPii: false,
    // Screenshots and the view tree can show guests' names and phone numbers.
    attachScreenshot: false,
    attachViewHierarchy: false,
    beforeSend: cleanEvent,
    beforeBreadcrumb: cleanBreadcrumb,
  });
  // Owners are known by id only, and the account they act on is a tag.
  useSession.subscribe((s) => {
    Sentry.setUser(s.onboarded && s.userId ? { id: s.userId } : null);
    Sentry.setTag("account_id", s.onboarded && s.accountId ? s.accountId : undefined);
  });
}

/** Reports a failed query or mutation when it is worth a look. See isReportable. */
export function reportError(error: unknown) {
  if (!isReporting || !isReportable(error)) return;
  Sentry.captureException(error, {
    tags: error instanceof ApiError ? { status: error.status, request_id: error.requestId } : {},
  });
}

/**
 * Sends a test error, so a support check can confirm reporting works in a release build.
 * Returns the event id to look up in Sentry, or null when this build doesn't report.
 */
export function sendTestError(): string | null {
  if (!isReporting) return null;
  return Sentry.captureException(new Error("Test error from the StayKey app, sent on purpose"));
}

export const wrap = Sentry.wrap;
