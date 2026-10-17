"use client";

import type { Currency, PublicUnit, StayRules, UnitAvailability } from "@staykey/api-client";
import { useEffect, useRef } from "react";
import { BookingCard } from "@/components/booking-card";

type Props = {
  slug: string;
  baseRate: number;
  currency: Currency;
  units: PublicUnit[];
  availability: UnitAvailability[];
  rules: StayRules;
  today: string;
  takingBookings: boolean;
  bookingPageUrl: string;
  hostOrigin: string | null;
};

/**
 * Runs inside the widget iFrame. Reports its height to the host page so the frame never
 * scrolls, and forwards booking events for the host's analytics (see packages/widget).
 * Checkout opens in a top-level tab because card payments break inside iFrames.
 */
export function EmbedBridge({
  slug,
  baseRate,
  currency,
  units,
  availability,
  rules,
  today,
  takingBookings,
  bookingPageUrl,
  hostOrigin,
}: Props) {
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
      {takingBookings ? (
        <BookingCard
          compact
          slug={slug}
          baseRate={baseRate}
          currency={currency}
          units={units}
          availability={availability}
          rules={rules}
          today={today}
          from="widget"
          onCheckAvailability={(stay) => {
            window.parent.postMessage(
              { source: "staykey", type: "event", name: "dates_selected", detail: stay },
              target,
            );
            const url = new URL(bookingPageUrl);
            url.searchParams.set("unit", stay.unitId);
            url.searchParams.set("checkIn", stay.checkIn);
            url.searchParams.set("checkOut", stay.checkOut);
            url.searchParams.set("adults", String(stay.adults));
            url.searchParams.set("children", String(stay.children));
            url.searchParams.set("from", "widget");
            window.open(url.toString(), "_blank", "noopener");
          }}
        />
      ) : (
        <p className="rounded-card border border-cloud bg-snow p-5 text-graphite">
          Not taking bookings online right now.
        </p>
      )}
    </div>
  );
}
