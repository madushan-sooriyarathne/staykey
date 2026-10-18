import "server-only";
import {
  type Account,
  type BillingPeriod,
  createStayKeyClient,
  forwardVisitor,
  type OtpSent,
  type Plan,
  type PlanId,
  type SignIn,
  type Subscription,
  type WebCheckout,
} from "@staykey/api-client";
import { visitorAddress } from "./visitor";

// Only this site's server calls the API; the owner's browser holds httpOnly cookies and never
// sees a token.
const api = createStayKeyClient(process.env.STAYKEY_API_URL ?? "http://localhost:8080");
api.use(forwardVisitor(process.env.STAYKEY_FORWARD_SECRET, visitorAddress));

export type Result<T> = { ok: true; data: T } | { ok: false; status: number; message: string };

type ErrorBody = { message?: string } | undefined;

function result<T>(data: T | undefined, error: ErrorBody, response: Response): Result<T> {
  if (data !== undefined && response.ok) return { ok: true, data };
  return {
    ok: false,
    status: response.status,
    message:
      error?.message ?? `Something went wrong on our side (${response.status}). Please try again.`,
  };
}

const bearer = (token: string) => ({ Authorization: `Bearer ${token}` });

export async function sendCode(phone: string): Promise<Result<OtpSent>> {
  const { data, error, response } = await api.POST("/v1/auth/otp", { body: { phone } });
  return result(data, error, response);
}

export async function verifyCode(phone: string, code: string): Promise<Result<SignIn>> {
  const { data, error, response } = await api.POST("/v1/auth/verify", {
    body: { phone, code, deviceName: "StayKey billing page" },
  });
  return result(data, error, response);
}

export async function signOut(refreshToken: string): Promise<void> {
  await api.POST("/v1/auth/logout", { body: { refreshToken } });
}

/** The accounts the person owns. Only owners choose plans. */
export async function ownedAccounts(token: string): Promise<Result<Account[]>> {
  const { data, error, response } = await api.GET("/v1/accounts", { headers: bearer(token) });
  const r = result(data, error, response);
  return r.ok ? { ok: true, data: r.data.items.filter((a) => a.role === "owner") } : r;
}

export async function subscription(token: string, account: string): Promise<Result<Subscription>> {
  const { data, error, response } = await api.GET("/v1/subscription", {
    headers: { ...bearer(token), "X-Account-Id": account },
  });
  return result(data, error, response);
}

export async function plans(): Promise<Plan[]> {
  const { data, response } = await api.GET("/v1/public/plans");
  if (!data) throw new Error(`StayKey API returned ${response.status} for plans`);
  return data.items;
}

export async function checkout(
  token: string,
  account: string,
  plan: PlanId,
  period: BillingPeriod,
): Promise<Result<WebCheckout>> {
  const { data, error, response } = await api.POST("/v1/subscription/checkout", {
    headers: { ...bearer(token), "X-Account-Id": account },
    body: { plan, period },
  });
  return result(data, error, response);
}

/** Confirms an email address with the token from its link. 404 means expired or used. */
export async function confirmEmail(token: string): Promise<Result<{ email: string }>> {
  const { data, error, response } = await api.POST("/v1/public/email-confirmation", {
    body: { token },
  });
  return result(data, error, response);
}
