import type { PublicProperty } from "@staykey/api-client";
import { formatRange } from "@/lib/dates";

type Props = {
  property: Pick<PublicProperty, "name" | "pausedReason" | "whatsapp">;
  /** Dates the guest already picked, carried into the message. */
  checkIn?: string;
  checkOut?: string;
  compact?: boolean;
};

/** wa.me wants the number as digits only. */
function whatsappLink(phone: string, text: string): string {
  return `https://wa.me/${phone.replace(/\D/g, "")}?text=${encodeURIComponent(text)}`;
}

/**
 * Stands in for the booking card when the page takes no bookings. When the owner's plan has
 * lapsed, guests can still reach them on WhatsApp.
 */
export function NotTakingBookings({ property, checkIn, checkOut, compact = false }: Props) {
  if (property.pausedReason === "lapsed" && property.whatsapp) {
    const dates = checkIn && checkOut ? ` for ${formatRange(checkIn, checkOut)}` : "";
    const href = whatsappLink(
      property.whatsapp,
      `Hi, I'd like to book ${property.name}${dates}. Is it available?`,
    );
    return (
      <div className="flex flex-col gap-3 rounded-card border border-cloud bg-snow p-5 text-graphite">
        {!compact && <p>To book {property.name}, message the owner directly.</p>}
        <a
          href={href}
          target="_blank"
          rel="noopener"
          className="rounded-button bg-obsidian px-5 py-4 text-center text-snow shadow-button"
        >
          Message {property.name} on WhatsApp
        </a>
      </div>
    );
  }
  return (
    <p className="rounded-card border border-cloud bg-snow p-5 text-graphite">
      {compact
        ? "Not taking bookings online right now."
        : `${property.name} isn't taking bookings online right now. Please contact the property directly.`}
    </p>
  );
}
