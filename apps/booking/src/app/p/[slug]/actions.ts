"use server";

import type { GuestPaymentMethod, Quote } from "@staykey/api-client";
import { refresh } from "next/cache";
import { redirect } from "next/navigation";
import { bookStay, holdStay, type Problem, quoteStay, uploadSlip } from "@/lib/api";
import { isDay } from "@/lib/dates";
import { pagePath } from "@/lib/paths";

// These run on the booking app's server and only call the public API, which checks every
// rule itself. Anyone can post to them, so they trust nothing a form sends.

export type QuoteInput = {
  slug: string;
  unitId: string;
  checkIn: string;
  checkOut: string;
  adults: number;
  children: number;
  extras?: string[];
  promo?: string;
};

export type QuoteState = { quote?: Quote; problem?: Problem };

/** Prices a stay for the booking card and the checkout. */
export async function quote(input: QuoteInput): Promise<QuoteState> {
  if (!isDay(input.checkIn) || !isDay(input.checkOut)) return {};
  const res = await quoteStay(input.slug, {
    unitId: input.unitId,
    checkIn: input.checkIn,
    checkOut: input.checkOut,
    adults: input.adults,
    children: input.children,
    extras: input.extras,
    promo: input.promo || undefined,
  });
  return res.ok ? { quote: res.data } : { problem: res.problem };
}

export type FormState = { problem?: Problem };

function text(form: FormData, name: string): string {
  const v = form.get(name);
  return typeof v === "string" ? v.trim() : "";
}

function count(form: FormData, name: string, fallback: number): number {
  const n = Number.parseInt(text(form, name), 10);
  return Number.isFinite(n) ? n : fallback;
}

/** Holds the chosen nights and opens the checkout. */
export async function hold(_: FormState, form: FormData): Promise<FormState> {
  const slug = text(form, "slug");
  const stay = {
    unitId: text(form, "unitId"),
    checkIn: text(form, "checkIn"),
    checkOut: text(form, "checkOut"),
  };
  if (!isDay(stay.checkIn) || !isDay(stay.checkOut)) {
    return { problem: { message: "Pick your check-in and check-out dates." } };
  }
  const replaces = text(form, "replaces");
  const res = await holdStay(slug, { ...stay, replaces: replaces || undefined });
  if (!res.ok) return { problem: res.problem };

  const params = new URLSearchParams({
    ...stay,
    hold: res.data.holdId,
    expires: res.data.expiresAt,
    adults: String(count(form, "adults", 2)),
    children: String(count(form, "children", 0)),
    from: text(form, "from") === "widget" ? "widget" : "page",
  });
  redirect(`${await pagePath(slug, "/book")}?${params}`);
}

function paymentMethod(value: string): GuestPaymentMethod | undefined {
  return value === "bank" || value === "property" ? value : undefined;
}

/** Books the stay and opens the guest's stay page, whose link carries their token. */
export async function book(_: FormState, form: FormData): Promise<FormState> {
  const slug = text(form, "slug");
  const holdId = text(form, "holdId");
  const method = paymentMethod(text(form, "paymentMethod"));
  if (!method) {
    return { problem: { field: "paymentMethod", message: "Choose how you'll pay." } };
  }
  const res = await bookStay(
    slug,
    {
      holdId: holdId || undefined,
      unitId: text(form, "unitId"),
      checkIn: text(form, "checkIn"),
      checkOut: text(form, "checkOut"),
      adults: count(form, "adults", 1),
      children: count(form, "children", 0),
      extras: form.getAll("extras").filter((v): v is string => typeof v === "string"),
      promo: text(form, "promo") || undefined,
      guest: {
        name: text(form, "name"),
        phone: text(form, "phone") || undefined,
        email: text(form, "email") || undefined,
        country: text(form, "country").toUpperCase() || undefined,
      },
      note: text(form, "note") || undefined,
      paymentMethod: method,
      source: text(form, "from") === "widget" ? "widget" : "page",
    },
    text(form, "key"),
  );
  if (!res.ok) return { problem: res.problem };
  const { booking, token } = res.data;
  redirect(`${await pagePath(slug, `/stay/${booking.id}`)}?t=${encodeURIComponent(token)}`);
}

/** Sends a photo of a bank slip for the guest's booking. */
export async function sendSlip(_: FormState, form: FormData): Promise<FormState> {
  const file = form.get("file");
  if (!(file instanceof File) || file.size === 0) {
    return { problem: { field: "file", message: "Choose a photo of the slip." } };
  }
  const amount = Math.round(Number.parseFloat(text(form, "amount")) * 100);
  if (!Number.isFinite(amount) || amount < 1) {
    return { problem: { field: "amount", message: "Enter the amount you transferred." } };
  }
  const res = await uploadSlip(text(form, "bookingId"), text(form, "token"), file, amount);
  if (!res.ok) return { problem: res.problem };
  refresh();
  return {};
}
