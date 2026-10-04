import { Linking } from "react-native";
import { money } from "@/components/kit";
import { formatShort } from "@/data/dates";
import { clock } from "@/data/labels";
import { balanceOf } from "@/data/pricing";
import type { Booking, PropertyConfig } from "@/data/types";

const digits = (phone: string) => phone.replace(/[^\d]/g, "");

export function openWhatsApp(phone: string | undefined, text?: string) {
  const query = text ? `?text=${encodeURIComponent(text)}` : "";
  return Linking.openURL(`https://wa.me/${phone ? digits(phone) : ""}${query}`);
}

export function call(phone: string) {
  return Linking.openURL(`tel:${phone.replace(/\s/g, "")}`);
}

export function email(to: string, subject?: string, body?: string) {
  const params = new URLSearchParams();
  if (subject) params.set("subject", subject);
  if (body) params.set("body", body);
  const q = params.toString();
  return Linking.openURL(`mailto:${to}${q ? `?${q.replace(/\+/g, "%20")}` : ""}`);
}

/** Fills {placeholders} in a message template from a booking. */
export function fillTemplate(body: string, b: Booking | undefined, p: PropertyConfig): string {
  const firstName = b?.guest.name.split(" ")[0] ?? "Emma";
  const values: Record<string, string> = {
    guest_name: firstName,
    property_name: p.name,
    check_in: b ? formatShort(b.checkIn) : "Sun 4 Oct",
    check_out: b ? formatShort(b.checkOut) : "Thu 8 Oct",
    check_in_time: clock(p.checkIn),
    balance: b ? money(balanceOf(b), p.currency) : money(18000, p.currency),
    map_link: `https://maps.google.com/?q=${encodeURIComponent(`${p.name} ${p.location}`)}`,
    booking_link: p.bookingPageUrl,
  };
  return body.replace(/\{(\w+)\}/g, (m, key: string) => values[key] ?? m);
}
