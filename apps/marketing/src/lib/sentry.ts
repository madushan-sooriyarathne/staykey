import { cleanBreadcrumb, cleanEvent, reportingEnabled } from "@staykey/observe";

const dsn = process.env.NEXT_PUBLIC_SENTRY_DSN;
const environment = process.env.NEXT_PUBLIC_STAYKEY_ENVIRONMENT;

/**
 * Sentry options for the server and the browser. Only production builds with a DSN report, so
 * development and staging spend none of the free quota. Guests are never identified.
 */
export const sentryOptions = {
  dsn,
  environment,
  release: process.env.NEXT_PUBLIC_STAYKEY_RELEASE,
  enabled: reportingEnabled(environment, dsn),
  sendDefaultPii: false,
  beforeSend: cleanEvent,
  beforeBreadcrumb: cleanBreadcrumb,
};
