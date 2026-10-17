import { type Currency, formatMoney } from "@staykey/api-client";

// Written out on the server only, in Sri Lanka's time, so pages and their client parts agree.

const dateFormat = new Intl.DateTimeFormat("en-GB", {
  day: "numeric",
  month: "short",
  year: "numeric",
  timeZone: "Asia/Colombo",
});

/** "24 Oct 2026". */
export function formatDate(iso: string): string {
  return dateFormat.format(new Date(iso));
}

/** "Rs 5,900" for whole rupees. */
export function formatRupees(amountMinor: number, currency: Currency): string {
  return formatMoney(amountMinor, currency, "en-LK");
}
