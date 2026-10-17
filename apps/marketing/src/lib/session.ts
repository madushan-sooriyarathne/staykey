// Cookies for the web billing page. They are httpOnly and scoped to /billing, so nothing else on
// the site sends or reads them.

export const ACCESS_COOKIE = "staykey_billing_access";
export const REFRESH_COOKIE = "staykey_billing_refresh";
/** The account the owner last chose, so the done page knows which plan to show. */
export const ACCOUNT_COOKIE = "staykey_billing_account";

export function cookieOptions(expires?: Date) {
  return {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path: "/billing",
    expires,
  };
}
