import { ogImage, ogSize } from "@/lib/og";

export const alt =
  "StayKey: take direct bookings in 10 minutes. Built for villas and guesthouses in Sri Lanka.";
export const size = ogSize;
export const contentType = "image/png";

export default function Image() {
  return ogImage({
    headline: "Take direct bookings in 10 minutes",
    line: "A booking page, a website widget and one calendar for villas and guesthouses.",
    chip: "Built for Sri Lanka",
  });
}
