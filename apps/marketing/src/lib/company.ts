// Who runs StayKey, and the versions of its legal texts. The legal pages, the footer and the
// structured data read from here.

export const COMPANY = {
  name: "Advertaro (Private) Limited",
  registration: "PV00242164",
  address: "No. 18, Gnanathilake Road, Mount Lavinia, Sri Lanka",
  legalEmail: "hello@madushan.dev",
} as const;

export const SUPPORT = {
  email: "support@staykey.direct",
  whatsapp: "+94 71 688 0657",
  whatsappLink: "https://wa.me/94716880657",
} as const;

export const SITE_URL = "https://staykey.direct";

/**
 * Set to true by a person once a lawyer has reviewed the terms, the privacy policy and the
 * payout terms. Until then the pages say they are drafts and aren't indexed.
 */
export const LEGAL_REVIEWED = false;

export const LEGAL = {
  privacy: { version: "2026-10", updated: "2026-10-21" },
  terms: { version: "2026-10", updated: "2026-10-21" },
  // The app records this version when an owner accepts the payout terms.
  payouts: { version: "2026-10", updated: "2026-10-21" },
} as const;
