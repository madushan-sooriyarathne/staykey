// What StayKey's web apps and owner app may send to Sentry, and the rule that only production
// sends anything. It mirrors apps/api/internal/observe: no phones, emails, payment keys,
// tokens or NICs, no query strings, and no token path segments.

const REMOVED = "[removed]";

/** Sentry gets events only from production builds with a DSN, to keep the free quota for real errors. */
export function reportingEnabled(
  environment: string | undefined,
  dsn: string | undefined,
): boolean {
  return environment === "production" && Boolean(dsn);
}

// Keys and tokens go before the looser number patterns.
const SCRUBBERS: RegExp[] = [
  /sk_(?:live|test)_[A-Za-z0-9_-]+/g,
  /bearer\s+[A-Za-z0-9._~+/=-]+/gi,
  /eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+/g,
  /[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}/g,
  /\+\d[\d -]{6,16}\d/g,
  /\b0?7\d{8}\b/g,
  /\b\d{9}[VvXx]\b/g,
  /\b(?:19|20)\d{10}\b/g,
];

/** Replaces phone numbers, email addresses, payment keys, tokens and NIC numbers in s. */
export function scrub(s: string): string {
  return SCRUBBERS.reduce((out, re) => out.replace(re, REMOVED), s);
}

const TOKEN_PATHS: [RegExp, string][] = [
  [/\/join\/[^/?#]+/g, "/join/:token"],
  [/\/verify-email\/[^/?#]+/g, "/verify-email/:token"],
  [/\/ical\/[^/?#]+/g, "/ical/:file"],
];

/** Drops a URL's query and fragment, which carry guest and invite tokens, and token path segments. */
export function cleanUrl(raw: string): string {
  let out = raw.split(/[?#]/, 1)[0] ?? "";
  for (const [re, to] of TOKEN_PATHS) out = out.replace(re, to);
  return scrub(out);
}

type Dict = Record<string, unknown>;

/** The parts of a Sentry event StayKey touches; the rest passes through. */
export type EventLike = {
  message?: string;
  request?: {
    url?: string;
    method?: string;
    headers?: Record<string, string>;
    [k: string]: unknown;
  };
  user?: { id?: string | number; [k: string]: unknown };
  exception?: { values?: { value?: string; [k: string]: unknown }[] };
  breadcrumbs?: BreadcrumbLike[];
  tags?: Record<string, unknown>;
  contexts?: Dict;
  extra?: Dict;
  transaction?: string;
  [k: string]: unknown;
};

export type BreadcrumbLike = { message?: string; data?: Dict; [k: string]: unknown };

/**
 * Strips an event to what is safe to send. For beforeSend. It takes the SDK's own event type,
 * whose interfaces don't declare an index signature, and reads it through EventLike.
 */
export function cleanEvent<T extends object>(input: T): T {
  const event = input as EventLike;
  if (event.request) {
    const agent = event.request.headers?.["User-Agent"] ?? event.request.headers?.["user-agent"];
    event.request = {
      method: event.request.method,
      url: event.request.url ? cleanUrl(event.request.url) : undefined,
      ...(agent ? { headers: { "User-Agent": agent } } : {}),
    };
  }
  if (event.user) event.user = event.user.id !== undefined ? { id: event.user.id } : {};
  if (event.message) event.message = scrub(event.message);
  if (event.transaction) event.transaction = cleanUrl(event.transaction);
  for (const ex of event.exception?.values ?? []) {
    if (ex.value) ex.value = scrub(ex.value);
  }
  if (event.tags) {
    for (const [k, v] of Object.entries(event.tags)) {
      if (typeof v === "string") event.tags[k] = scrub(v);
    }
  }
  // The SDKs put the request path, query string included, in contexts such as nextjs.
  if (event.contexts) event.contexts = cleanValues(event.contexts);
  if (event.extra) event.extra = cleanValues(event.extra);
  if (event.breadcrumbs) {
    event.breadcrumbs = event.breadcrumbs.map((b) => cleanBreadcrumb(b));
  }
  return input;
}

/** Cleans a breadcrumb's message and data, and drops request and response bodies. For beforeBreadcrumb. */
export function cleanBreadcrumb<T extends object>(input: T): T {
  const crumb = input as BreadcrumbLike;
  if (crumb.message) crumb.message = scrub(crumb.message);
  if (crumb.data) {
    const data: Dict = {};
    for (const [k, v] of Object.entries(crumb.data)) {
      if (k === "body" || k === "data" || k === "input") continue;
      if (typeof v === "string")
        data[k] = k === "url" || k === "to" || k === "from" ? cleanUrl(v) : scrub(v);
      else data[k] = v;
    }
    crumb.data = data;
  }
  return input;
}

function isPlainObject(v: unknown): v is Dict {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

// Paths and URLs lose their query and token segments; any other string is scrubbed.
function cleanValues(dict: Dict): Dict {
  const out: Dict = {};
  for (const [k, v] of Object.entries(dict)) {
    if (typeof v === "string") out[k] = /^(?:\/|https?:\/\/)/.test(v) ? cleanUrl(v) : scrub(v);
    else if (isPlainObject(v)) out[k] = cleanValues(v);
    else out[k] = v;
  }
  return out;
}
