"use client";

import type { Currency } from "@staykey/api-client";
import { useEffect, useRef } from "react";
import { BookingCard } from "@/components/booking-card";

type Props = {
  baseRate: number;
  currency: Currency;
  bookingPageUrl: string;
  hostOrigin: string | null;
};

/**
 * Runs inside the widget iFrame. Reports its height to the host page so the frame never
 * scrolls, and forwards booking events for the host's analytics (see packages/widget).
 * Checkout opens in a top-level tab because card payments break inside iFrames.
 */
export function EmbedBridge({ baseRate, currency, bookingPageUrl, hostOrigin }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const target = hostOrigin ?? "*";

  useEffect(() => {
    const el = ref.current;
    if (!el || window.parent === window) return;
    const post = () =>
      window.parent.postMessage(
        { source: "staykey", type: "resize", height: el.scrollHeight },
        target,
      );
    const observer = new ResizeObserver(post);
    observer.observe(el);
    post();
    return () => observer.disconnect();
  }, [target]);

  return (
    <div ref={ref} className="staykey-embed p-1">
      <BookingCard
        compact
        baseRate={baseRate}
        currency={currency}
        onCheckAvailability={(dates) => {
          window.parent.postMessage(
            { source: "staykey", type: "event", name: "dates_selected", detail: dates },
            target,
          );
          const url = new URL(bookingPageUrl);
          url.searchParams.set("checkIn", dates.checkIn);
          url.searchParams.set("checkOut", dates.checkOut);
          url.searchParams.set("guests", String(dates.guests));
          window.open(url.toString(), "_blank", "noopener");
        }}
      />
    </div>
  );
}
