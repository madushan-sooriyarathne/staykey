import { sentryOptions } from "@/lib/sentry";

// The SDK loads once the browser is idle, so error reporting adds nothing to the first paint
// or to blocking time. An error thrown before then goes unreported.
if (sentryOptions.enabled) {
  const start = () => {
    import("@sentry/nextjs").then((Sentry) => Sentry.init(sentryOptions));
  };
  if ("requestIdleCallback" in window) window.requestIdleCallback(start);
  else setTimeout(start, 1);
}
