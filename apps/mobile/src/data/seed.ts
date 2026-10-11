import { addDays, today } from "./dates";
import { makeProperty, sampleMoney, uid } from "./defaults";
import type { PropertyConfig } from "./types";

/**
 * Sample property setups for the prototype tools. Sample stays, the team and their activity come
 * from cmd/seed on the server.
 */

const hoursAgo = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();

/** Adds the settings a lived-in property would have, without overriding what the owner chose. */
export function withSampleSettings(p: PropertyConfig): PropertyConfig {
  const m = (usd: number) => sampleMoney(usd, p.currency);
  return {
    ...p,
    description:
      p.description ||
      (p.bookingType === "entire"
        ? "Three-bedroom villa with a private pool, ten minutes from the beach."
        : "A small guesthouse with garden rooms, a short walk from the beach."),
    amenities: p.amenities.length ? p.amenities : ["Pool", "WiFi", "Air conditioning", "Kitchen"],
    extraGuest: p.extraGuest ?? (p.bookingType === "entire" ? { above: 4, amount: m(25) } : null),
    seasons: p.seasons.length
      ? p.seasons
      : [
          {
            id: uid("sea"),
            name: "Peak season",
            start: "12-15",
            end: "01-15",
            prices: Object.fromEntries(
              p.units.map((u) => [u.id, Math.round((u.rate * 1.75) / 1000) * 1000]),
            ),
            minNights: 3,
          },
          {
            id: uid("sea"),
            name: "High season",
            start: "01-16",
            end: "04-30",
            prices: Object.fromEntries(
              p.units.map((u) => [u.id, Math.round((u.rate * 1.45) / 1000) * 1000]),
            ),
          },
          {
            id: uid("sea"),
            name: "Monsoon",
            start: "05-01",
            end: "08-31",
            prices: Object.fromEntries(
              p.units.map((u) => [u.id, Math.round((u.rate * 0.8) / 1000) * 1000]),
            ),
          },
        ],
    lengthDiscounts: p.lengthDiscounts.length
      ? p.lengthDiscounts
      : [
          { nights: 7, percent: 10 },
          { nights: 28, percent: 25 },
        ],
    charges: p.charges
      .map((c) => (c.name === "Service charge" ? { ...c, enabled: true } : c))
      .concat(
        p.charges.some((c) => c.name === "Cleaning fee")
          ? []
          : [
              {
                id: uid("chg"),
                name: "Cleaning fee",
                kind: "fixed",
                amount: m(30),
                per: "stay",
                enabled: true,
              },
            ],
      ),
    extras: p.extras.length
      ? p.extras
      : [
          {
            id: "ext_transfer",
            name: "Airport transfer",
            price: m(40),
            per: "trip",
            onRequest: false,
            enabled: true,
          },
          {
            id: "ext_breakfast",
            name: "Breakfast",
            price: m(8),
            per: "guestNight",
            onRequest: false,
            enabled: true,
          },
          {
            id: "ext_tour",
            name: "Galle Fort tuk-tuk tour",
            price: m(25),
            per: "guest",
            onRequest: false,
            enabled: true,
          },
          {
            id: "ext_late",
            name: "Late check-out",
            price: m(30),
            per: "stay",
            onRequest: true,
            enabled: false,
          },
        ],
    promos: p.promos.length
      ? p.promos
      : [
          {
            id: uid("pro"),
            code: "RETURN10",
            kind: "percent",
            amount: 10,
            used: 4,
            limit: 20,
            note: "For returning guests",
          },
          {
            id: uid("pro"),
            code: "MONSOON50",
            kind: "fixed",
            amount: m(50),
            used: 0,
            minNights: 3,
            from: `${new Date().getFullYear() + 1}-05-01`,
            to: `${new Date().getFullYear() + 1}-08-31`,
          },
          {
            id: uid("pro"),
            code: "FRIENDS15",
            kind: "percent",
            amount: 15,
            used: 12,
            to: addDays(today(), -4),
          },
        ],
    payments: {
      ...p.payments,
      bank: {
        ...p.payments.bank,
        bankName: p.payments.bank.bankName || "Commercial Bank",
        accountName: p.payments.bank.accountName || "N. Perera",
        accountNumber: p.payments.bank.accountNumber || "8001234417",
      },
    },
    ical: [
      {
        id: uid("ical"),
        channel: "airbnb",
        url: "https://www.airbnb.com/calendar/ical/41829.ics",
        status: "error",
        lastSync: hoursAgo(2.3),
        error: "Airbnb didn't respond twice",
        upcoming: 1,
      },
      {
        id: uid("ical"),
        channel: "booking",
        url: "https://admin.booking.com/hotel/hoteladmin/ical.html?t=7f3a9c",
        status: "ok",
        lastSync: hoursAgo(0.5),
        upcoming: 1,
      },
    ],
  };
}

/** A sample villa for team members who join someone else's property. */
export function sampleVilla(): PropertyConfig {
  const id = "prop_sample_kv";
  return withSampleSettings(
    makeProperty({
      id,
      name: "Kingfisher Villa",
      slug: "kingfisher",
      bookingPageUrl: "https://kingfisher.staykey.direct",
      bookingType: "entire",
      currency: "USD",
      location: "Unawatuna, Galle",
      units: [
        {
          id: `${id}_u1`,
          name: "Kingfisher Villa",
          sleeps: 6,
          beds: "3 bedrooms",
          rate: 18000,
          weekendRate: 22000,
        },
      ],
    }),
  );
}

/** A sample guesthouse with rooms and a linked whole-house unit. */
export function sampleGuesthouse(): PropertyConfig {
  const id = `prop_sample_cb_${Date.now().toString(36)}`;
  return withSampleSettings(
    makeProperty({
      id,
      name: "Coral Bay House",
      slug: "coralbay",
      bookingPageUrl: "https://coralbay.staykey.direct",
      bookingType: "rooms",
      currency: "USD",
      location: "Mirissa, Matara",
      policy: "flexible",
      units: [
        {
          id: `${id}_ocean`,
          name: "Ocean Room",
          sleeps: 2,
          beds: "1 king bed",
          rate: 9000,
          weekendRate: 10000,
        },
        { id: `${id}_garden`, name: "Garden Room", sleeps: 2, beds: "2 single beds", rate: 7500 },
        {
          id: `${id}_family`,
          name: "Family Suite",
          sleeps: 4,
          beds: "1 king and 2 single beds",
          rate: 13000,
        },
        {
          id: `${id}_whole`,
          name: "Whole house",
          sleeps: 8,
          beds: "Books all three rooms together",
          rate: 28000,
          linkedUnitIds: [`${id}_ocean`, `${id}_garden`, `${id}_family`],
        },
      ],
    }),
  );
}
