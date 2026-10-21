import { cachedPlans } from "@/lib/api";
import { formatRupees } from "@/lib/format";
import { ogImage, ogSize } from "@/lib/og";

export const alt = "StayKey pricing: one flat price for your property, no commission on bookings.";
export const size = ogSize;
export const contentType = "image/png";

export default async function Image() {
  const villa = (await cachedPlans())?.[0];
  return ogImage({
    headline: "One flat price, no commission",
    line: villa
      ? `From ${formatRupees(villa.monthly, villa.currency)} a month for a villa. Free until your first direct booking.`
      : "Plans for one villa up to 20 rooms. Free until your first direct booking.",
    chip: "Every feature on every plan",
  });
}
