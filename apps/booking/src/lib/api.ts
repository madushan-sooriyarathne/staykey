import "server-only";
import {
  type Availability,
  createStayKeyClient,
  forwardVisitor,
  type GuestBooking,
  type GuestBookingCreated,
  type GuestBookingRequest,
  type Hold,
  type PublicProperty,
  type PublicQuoteRequest,
  type Quote,
} from "@staykey/api-client";
import { visitorAddress } from "./visitor";

// The booking app is the only caller: guests' browsers never reach the API, so its address
// stays private. The guest's address goes along, so rate limits count guests and not this app.
const api = createStayKeyClient(process.env.STAYKEY_API_URL ?? "http://localhost:8080");
api.use(forwardVisitor(process.env.STAYKEY_FORWARD_SECRET, visitorAddress));

/** What went wrong with a guest's request, in words for the guest. */
export type Problem = { message: string; field?: string; code?: string };
export type Result<T> = { ok: true; data: T } | { ok: false; problem: Problem };

type ErrorBody = { code?: string; message?: string; field?: string } | undefined;

function problem(error: ErrorBody, status: number): Problem {
  if (error?.message) return { message: error.message, field: error.field, code: error.code };
  return { message: `Something went wrong on our side (${status}). Please try again.` };
}

function result<T>(data: T | undefined, error: ErrorBody, response: Response): Result<T> {
  if (data !== undefined && response.ok) return { ok: true, data };
  return { ok: false, problem: problem(error, response.status) };
}

/** Loads a property's public booking page details, or null when the slug is unknown. */
export async function getPublicProperty(slug: string): Promise<PublicProperty | null> {
  const { data, response } = await api.GET("/v1/public/properties/{slug}", {
    params: { path: { slug } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`StayKey API returned ${response.status} for property "${slug}"`);
  return data;
}

/** Nights each unit can't be booked in [from, to), and days guests can't arrive. */
export async function getAvailability(
  slug: string,
  from: string,
  to: string,
): Promise<Availability> {
  const { data, response } = await api.GET("/v1/public/properties/{slug}/availability", {
    params: { path: { slug }, query: { from, to } },
  });
  if (!data)
    throw new Error(`StayKey API returned ${response.status} for availability of "${slug}"`);
  return data;
}

export async function quoteStay(slug: string, body: PublicQuoteRequest): Promise<Result<Quote>> {
  const { data, error, response } = await api.POST("/v1/public/properties/{slug}/quote", {
    params: { path: { slug } },
    body,
  });
  return result(data, error, response);
}

export async function holdStay(
  slug: string,
  body: { unitId: string; checkIn: string; checkOut: string; replaces?: string },
): Promise<Result<Hold>> {
  const { data, error, response } = await api.POST("/v1/public/properties/{slug}/holds", {
    params: { path: { slug } },
    body,
  });
  return result(data, error, response);
}

export async function bookStay(
  slug: string,
  body: GuestBookingRequest,
  idempotencyKey: string,
): Promise<Result<GuestBookingCreated>> {
  const { data, error, response } = await api.POST("/v1/public/properties/{slug}/bookings", {
    params: { path: { slug }, header: { "Idempotency-Key": idempotencyKey } },
    body,
  });
  return result(data, error, response);
}

/** A guest's own booking, or null when the id or token is wrong. */
export async function getGuestBooking(id: string, token: string): Promise<GuestBooking | null> {
  const { data, response } = await api.GET("/v1/public/bookings/{bookingId}", {
    params: { path: { bookingId: id }, header: { "X-Booking-Token": token } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`StayKey API returned ${response.status} for a guest booking`);
  return data;
}

const SLIP_TYPES = ["image/jpeg", "image/png", "image/webp", "image/heic"] as const;
type SlipType = (typeof SLIP_TYPES)[number];

function isSlipType(type: string): type is SlipType {
  return SLIP_TYPES.some((t) => t === type);
}

/** Uploads a bank slip to the presigned link the API gives, then sends it to the owner. */
export async function uploadSlip(
  id: string,
  token: string,
  file: File,
  amount: number,
): Promise<Result<GuestBooking>> {
  const contentType = file.type;
  if (!isSlipType(contentType)) {
    return {
      ok: false,
      problem: { field: "file", message: "Use a photo of the slip: JPEG, PNG, WebP or HEIC." },
    };
  }
  const params = { path: { bookingId: id }, header: { "X-Booking-Token": token } };
  const ticket = await api.POST("/v1/public/bookings/{bookingId}/uploads", {
    params,
    body: { contentType, size: file.size },
  });
  if (!ticket.data) return { ok: false, problem: problem(ticket.error, ticket.response.status) };

  const put = await fetch(ticket.data.uploadUrl, {
    method: ticket.data.method,
    headers: ticket.data.headers,
    body: Buffer.from(await file.arrayBuffer()),
  });
  if (!put.ok) {
    return { ok: false, problem: { message: "The slip didn't upload. Please try again." } };
  }

  const { data, error, response } = await api.POST("/v1/public/bookings/{bookingId}/slips", {
    params,
    body: { key: ticket.data.key, amount },
  });
  return result(data, error, response);
}
