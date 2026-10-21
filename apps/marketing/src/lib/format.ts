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

/** "Rs 5,900" for whole rupees, as prices are written in Sri Lanka; other currencies as usual. */
export function formatRupees(amountMinor: number, currency: Currency): string {
  if (currency !== "LKR") return formatMoney(amountMinor, currency, "en-LK");
  const amount = amountMinor / 100;
  return new Intl.NumberFormat("en-LK", {
    style: "currency",
    currency,
    currencyDisplay: "narrowSymbol",
    maximumFractionDigits: Number.isInteger(amount) ? 0 : 2,
  }).format(amount);
}
