import type { PayoutEntryKind, PayoutStatus } from "@staykey/api-client";

/** The payout terms version owners accept. Mirrors domain.PayoutTermsVersion in apps/api. */
export const PAYOUT_TERMS_VERSION = "2026-10";

/** Strips the spaces and dashes people type in NIC and account numbers. */
export function stripSeparators(s: string): string {
  return s.replace(/[\s-]/g, "");
}

/** A Sri Lankan NIC: 9 digits and V or X, or 12 digits. Mirrors domain.ValidNIC. */
export function isValidNIC(raw: string): boolean {
  const s = stripSeparators(raw).toUpperCase();
  return /^\d{9}[VX]$/.test(s) || /^\d{12}$/.test(s);
}

/** 6 to 20 digits once separators are gone, as the API checks. */
export function isValidAccountNumber(raw: string): boolean {
  return /^\d{6,20}$/.test(stripSeparators(raw));
}

const ENTRY_LABEL: Record<PayoutEntryKind, string> = {
  payment: "Card payment",
  processing: "StayKey processing charge",
  refund: "Card refund",
  refund_returned: "Refund that failed",
  adjustment: "Adjustment",
  payout_returned: "Returned payout",
};

export function entryLabel(kind: PayoutEntryKind): string {
  return ENTRY_LABEL[kind];
}

const STATUS_LABEL: Record<PayoutStatus, string> = {
  prepared: "On its way",
  paid: "Paid",
  failed: "Didn't go through",
};

export function payoutStatusLabel(status: PayoutStatus): string {
  return STATUS_LABEL[status];
}
