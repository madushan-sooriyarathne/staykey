import createClient, { type ClientOptions, type Middleware } from "openapi-fetch";
import type { components, paths } from "./schema";

export type { Middleware } from "openapi-fetch";
export type { components, paths } from "./schema";

type Schemas = components["schemas"];
export type Property = Schemas["Property"];
export type NewProperty = Schemas["NewProperty"];
export type PublicProperty = Schemas["PublicProperty"];
export type BookingType = Schemas["BookingType"];
export type Currency = Schemas["Currency"];
export type ApiError = Schemas["Error"];
export type Health = Schemas["Health"];
export type User = Schemas["User"];
export type UserPatch = Schemas["UserPatch"];
export type Account = Schemas["Account"];
export type Role = Schemas["Role"];
export type Me = Schemas["Me"];
export type OtpSent = Schemas["OtpSent"];
export type SignIn = Schemas["SignIn"];
export type AuthTokens = Schemas["AuthTokens"];
export type PropertyPatch = Schemas["PropertyPatch"];
export type UploadTicket = Schemas["UploadTicket"];
export type Booking = Schemas["Booking"];
export type NewBooking = Schemas["NewBooking"];
export type BookingPatch = Schemas["BookingPatch"];
export type Transition = Schemas["Transition"];
export type Cancellation = Schemas["Cancellation"];
export type NewPayment = Schemas["NewPayment"];
export type Quote = Schemas["Quote"];
export type ConflictError = Schemas["ConflictError"];
export type Block = Schemas["Block"];
export type NewBlock = Schemas["NewBlock"];
export type Calendar = Schemas["Calendar"];
export type RateOverrideInput = Schemas["RateOverrideInput"];
export type Team = Schemas["Team"];
export type TeamMember = Schemas["TeamMember"];
export type Invite = Schemas["Invite"];
export type NewInvite = Schemas["NewInvite"];
export type MemberRole = Schemas["MemberRole"];
export type MemberPatch = Schemas["MemberPatch"];
export type InvitePreview = Schemas["InvitePreview"];
export type Activity = Schemas["Activity"];
export type ActivityKind = Schemas["ActivityKind"];
export type ActivityPage = Schemas["ActivityPage"];
export type Template = Schemas["Template"];
export type TemplatePatch = Schemas["TemplatePatch"];
export type TemplateChannel = Schemas["TemplateChannel"];
export type NotificationEvent = Schemas["NotificationEvent"];
export type NotificationPref = Schemas["NotificationPref"];
export type StayRules = Schemas["StayRules"];
export type PublicUnit = Schemas["PublicUnit"];
export type PublicExtra = Schemas["PublicExtra"];
export type Availability = Schemas["Availability"];
export type UnitAvailability = Schemas["UnitAvailability"];
export type PublicQuoteRequest = Schemas["PublicQuoteRequest"];
export type Hold = Schemas["Hold"];
export type GuestPaymentMethod = Schemas["GuestPaymentMethod"];
export type GuestBookingRequest = Schemas["GuestBookingRequest"];
export type GuestBookingCreated = Schemas["GuestBookingCreated"];
export type GuestBooking = Schemas["GuestBooking"];
export type Subscription = Schemas["Subscription"];
export type PlanId = Schemas["PlanId"];
export type BillingPeriod = Schemas["BillingPeriod"];
export type Plan = Schemas["Plan"];
export type WebCheckout = Schemas["WebCheckout"];
export type Payment = Schemas["Payment"];
export type PaymentSettings = Schemas["PaymentSettings"];
export type PublicCard = Schemas["PublicCard"];
export type GuestCard = Schemas["GuestCard"];
export type CardOption = Schemas["CardOption"];
export type CardPurpose = Schemas["CardPurpose"];
export type CardCheckout = Schemas["CardCheckout"];
export type CardMode = Schemas["CardMode"];
export type CardPayments = Schemas["CardPayments"];
export type CardConnection = Schemas["CardConnection"];

/** Header that names the account an owner route acts on. */
export const ACCOUNT_HEADER = "X-Account-Id";

export const PRODUCTION_API_URL = "https://api.staykey.direct";

/** Typed client for the StayKey API, generated from packages/api-spec/openapi.yaml. */
export function createStayKeyClient(baseUrl: string = PRODUCTION_API_URL, options?: ClientOptions) {
  return createClient<paths>({ ...options, baseUrl });
}

export type StayKeyClient = ReturnType<typeof createStayKeyClient>;

/**
 * For a site that calls the API from its own server: passes the visitor's address on, with the
 * secret the API shares with that site, so rate limits count the visitor rather than the site.
 * Does nothing without a secret or an address.
 */
export function forwardVisitor(
  secret: string | undefined,
  visitorAddress: () => Promise<string | undefined>,
): Middleware {
  return {
    async onRequest({ request }) {
      if (!secret) return undefined;
      const address = await visitorAddress();
      if (!address) return undefined;
      request.headers.set("X-StayKey-Forward-Key", secret);
      request.headers.set("X-StayKey-Guest-IP", address);
      return request;
    },
  };
}

/**
 * What a card is charged in rupee cents for an amount in minor units, at a rate in rupee cents
 * per 1.00 of the currency. Rounds up, like the API. No rate means the amount is in rupees.
 */
export function toRupees(amountMinor: number, rate?: number): number {
  if (!rate || rate <= 0) return amountMinor;
  return Math.ceil((amountMinor * rate) / 100);
}

/** Formats an amount in minor units, for example 18000 USD as "$180". */
export function formatMoney(amountMinor: number, currency: Currency, locale = "en-US"): string {
  const amount = amountMinor / 100;
  return new Intl.NumberFormat(locale, {
    style: "currency",
    currency,
    maximumFractionDigits: Number.isInteger(amount) ? 0 : 2,
  }).format(amount);
}

/**
 * Suggests a booking page slug from a property name, for example "Kingfisher Villa" becomes
 * "kingfisher-villa". Mirrors domain.Slugify in apps/api so suggestions match what the API accepts.
 */
export function suggestSlug(name: string): string {
  const slug = name
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, "-")
    .replace(/^-+|-+$/g, "");
  return slug.slice(0, 40).replace(/-+$/, "");
}

export const SLUG_PATTERN = /^[a-z0-9](?:[a-z0-9-]{1,38}[a-z0-9])$/;
