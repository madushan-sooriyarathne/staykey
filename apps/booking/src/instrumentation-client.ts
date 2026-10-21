import * as Sentry from "@sentry/nextjs";

import { sentryOptions } from "@/lib/sentry";

if (sentryOptions.enabled) Sentry.init(sentryOptions);
