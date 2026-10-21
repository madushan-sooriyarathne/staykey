import * as Sentry from "@sentry/nextjs";

import { sentryOptions } from "@/lib/sentry";

export function register() {
  if (sentryOptions.enabled) Sentry.init(sentryOptions);
}

export const onRequestError = Sentry.captureRequestError;
