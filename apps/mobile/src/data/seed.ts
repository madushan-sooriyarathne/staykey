import { addDays, today } from "./dates";
import { makeProperty, sampleMoney, uid } from "./defaults";
import { quote } from "./pricing";
import type {
  Activity,
  Block,
  Booking,
  BookingStatus,
  Payment,
  PropertyConfig,
  Source,
  TeamMember,
} from "./types";

/**
 * Sample data for the prototype, generated relative to today so Today, Calendar and Bookings
 * always have something to show. Replace with API data once bookings move server side.
 */

const hoursAgo = (h: number) => new Date(Date.now() - h * 3_600_000).toISOString();
const daysAgo = (d: number, hour = 10, minute = 0) => {
  const x = new Date();
  x.setDate(x.getDate() - d);
  x.setHours(hour, minute, 0, 0);
  return x.toISOString();
};

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

type Plan = {
  u: number;
  from: number;
  nights: number;
  source: Source;
  status: BookingStatus;
  name?: string;
  phone?: string;
  email?: string;
  country?: string;
  adults?: number;
  children?: number;
  extras?: string[];
  paid?: "full" | "deposit" | "none";
  method?: Payment["method"];
  slip?: boolean;
  guestNote?: string;
  ownerNote?: string;
  createdDaysAgo?: number;
  expiresInHours?: number;
};

const PLANS: Plan[] = [
  // Main timeline, on the first unit.
  {
    u: 0,
    from: -14,
    nights: 4,
    source: "page",
    status: "checked_out",
    name: "Hannah Clarke",
    country: "GB",
    paid: "full",
    method: "card",
    createdDaysAgo: 40,
  },
  {
    u: 0,
    from: -3,
    nights: 3,
    source: "widget",
    status: "checked_in",
    name: "Sofia Rossi",
    phone: "+39 347 555 0142",
    email: "sofia.rossi@example.com",
    country: "IT",
    paid: "full",
    method: "card",
    createdDaysAgo: 21,
  },
  {
    u: 0,
    from: 0,
    nights: 4,
    source: "page",
    status: "confirmed",
    name: "Emma Larsen",
    phone: "+44 7700 900123",
    email: "emma.larsen@example.com",
    country: "GB",
    extras: ["ext_transfer"],
    paid: "full",
    method: "card",
    guestNote: "We land at 11:30. Could we check in a little early?",
    ownerNote: "Leave pool towels out",
    createdDaysAgo: 10,
  },
  { u: 0, from: 6, nights: 3, source: "booking", status: "confirmed", createdDaysAgo: 1 },
  {
    u: 0,
    from: 12,
    nights: 3,
    source: "page",
    status: "requested",
    name: "Lukas Weber",
    phone: "+49 151 2345 6789",
    email: "lukas.weber@example.com",
    country: "DE",
    createdDaysAgo: 0,
    expiresInHours: 18,
  },
  { u: 0, from: 18, nights: 5, source: "airbnb", status: "confirmed", createdDaysAgo: 6 },
  {
    u: 0,
    from: 24,
    nights: 3,
    source: "whatsapp",
    status: "confirmed",
    name: "Daniel Fernando",
    phone: "+94 77 123 4567",
    country: "LK",
    adults: 4,
    paid: "deposit",
    method: "bank",
    slip: true,
    createdDaysAgo: 5,
  },
  {
    u: 0,
    from: 29,
    nights: 3,
    source: "page",
    status: "confirmed",
    name: "Mia Schneider",
    phone: "+49 160 555 0199",
    email: "mia.schneider@example.com",
    country: "DE",
    paid: "full",
    method: "card",
    createdDaysAgo: 12,
  },
  {
    u: 0,
    from: 40,
    nights: 3,
    source: "page",
    status: "cancelled",
    name: "Tom Becker",
    email: "tom.becker@example.com",
    country: "DE",
    createdDaysAgo: 20,
  },

  // Second and third units, for guesthouses.
  {
    u: 1,
    from: -6,
    nights: 4,
    source: "page",
    status: "checked_out",
    name: "Chen Wei",
    country: "SG",
    paid: "full",
    method: "card",
    createdDaysAgo: 30,
  },
  {
    u: 1,
    from: 0,
    nights: 3,
    source: "whatsapp",
    status: "confirmed",
    name: "Arjun Mehta",
    phone: "+91 98200 55501",
    country: "IN",
    paid: "deposit",
    method: "bank",
    slip: true,
    createdDaysAgo: 8,
  },
  { u: 1, from: 8, nights: 4, source: "booking", status: "confirmed", createdDaysAgo: 3 },
  {
    u: 2,
    from: 2,
    nights: 3,
    source: "widget",
    status: "awaiting_payment",
    name: "Olivia Brown",
    email: "olivia.brown@example.com",
    country: "AU",
    createdDaysAgo: 1,
  },
  {
    u: 2,
    from: 6,
    nights: 2,
    source: "page",
    status: "requested",
    name: "Priya Nair",
    phone: "+91 99000 12345",
    country: "IN",
    adults: 2,
    children: 1,
    createdDaysAgo: 0,
    expiresInHours: 20,
  },
];

/** Different guests for a second sample property, so the two don't share names. */
const ALT_NAMES: Record<string, string> = {
  "Emma Larsen": "Chloe Martin",
  "Sofia Rossi": "Elena Garcia",
  "Hannah Clarke": "Olga Petrova",
  "Lukas Weber": "Tomas Novak",
  "Daniel Fernando": "Nimal Wijesinghe",
  "Mia Schneider": "Julia Nowak",
  "Tom Becker": "Ben Carter",
  "Anna Svensson": "Freya Olsen",
  "Kavindu Silva": "Ruvini Fonseka",
  "Lena Hoffmann": "Clara Weiss",
  "James Wilson": "Oliver Grant",
  "Ayesha Khan": "Sana Malik",
  "Marco Bianchi": "Luca Romano",
  "Sarah Miller": "Emily Turner",
};

export function sampleData(
  p: PropertyConfig,
  alt = false,
): {
  bookings: Booking[];
  blocks: Block[];
  activity: Activity[];
} {
  const result = buildSample(p);
  if (!alt) return result;
  const rename = (text: string) =>
    Object.entries(ALT_NAMES).reduce((t, [a, b]) => t.replaceAll(a, b), text);
  return {
    ...result,
    bookings: result.bookings.map((b) => ({
      ...b,
      guest: {
        ...b.guest,
        name: rename(b.guest.name),
        email: b.guest.email
          ? `${rename(b.guest.name).toLowerCase().replace(/\s+/g, ".")}@example.com`
          : undefined,
      },
    })),
    activity: result.activity.map((a) => ({ ...a, title: rename(a.title) })),
  };
}

function buildSample(p: PropertyConfig): {
  bookings: Booking[];
  blocks: Block[];
  activity: Activity[];
} {
  const base = today();
  const bookings: Booking[] = [];
  const physical = p.units.filter((u) => !u.linkedUnitIds?.length);
  const units = physical.length ? physical : p.units;
  let ref = 2030;

  for (const plan of PLANS) {
    const unit = units[plan.u];
    if (!unit) continue;
    const checkIn = addDays(base, plan.from);
    const checkOut = addDays(checkIn, plan.nights);
    const ota = ["airbnb", "booking", "agoda", "expedia"].includes(plan.source);
    const adults = Math.min(plan.adults ?? 2, unit.sleeps);
    const extras = (plan.extras ?? []).filter((id) => p.extras.some((e) => e.id === id));
    const q = ota
      ? { lines: [], total: 0 }
      : quote(p, {
          unitId: unit.id,
          from: checkIn,
          to: checkOut,
          adults,
          children: plan.children,
          extras,
        });
    const total = q.total;
    const paidAt = daysAgo(plan.createdDaysAgo ?? 3, 14, 20);
    const payments: Payment[] = [];
    if (plan.paid === "full")
      payments.push({ id: uid("pay"), amount: total, method: plan.method ?? "card", at: paidAt });
    if (plan.paid === "deposit") {
      const deposit = Math.round((total * p.depositPercent) / 100 / 100) * 100;
      payments.push({ id: uid("pay"), amount: deposit, method: plan.method ?? "bank", at: paidAt });
    }
    const paid = payments.reduce((n, x) => n + x.amount, 0);
    ref += 1 + (bookings.length % 3);

    const b: Booking = {
      id: uid("bk"),
      ref: `${p.name
        .split(/\s+/)
        .map((w) => w[0]?.toUpperCase())
        .join("")
        .slice(0, 2)}-${ref}`,
      propertyId: p.id,
      unitId: unit.id,
      source: plan.source,
      status: plan.status,
      guest: ota
        ? { name: plan.source === "airbnb" ? "Airbnb stay" : "Booking.com stay" }
        : {
            name: plan.name ?? "Guest",
            phone: plan.phone,
            email: plan.email,
            country: plan.country,
          },
      adults,
      children: plan.children ?? 0,
      checkIn,
      checkOut,
      lines: q.lines,
      total,
      payments,
      extras,
      guestNote: plan.guestNote,
      ownerNote: plan.ownerNote,
      createdAt: daysAgo(plan.createdDaysAgo ?? 3, 9, 30),
      requestExpiresAt: plan.expiresInHours
        ? new Date(Date.now() + plan.expiresInHours * 3_600_000).toISOString()
        : undefined,
      slip: plan.slip
        ? { amount: total - paid, at: daysAgo(1, 21, 12), status: "pending" }
        : undefined,
      cancel:
        plan.status === "cancelled"
          ? { reason: "Guest changed plans", refund: 0, at: daysAgo(3, 16, 5) }
          : undefined,
    };
    bookings.push(b);
  }

  // A few months of past stays so Insights has history to show.
  const primary = units[0];
  const rotation: Source[] = [
    "page",
    "booking",
    "airbnb",
    "whatsapp",
    "widget",
    "page",
    "booking",
    "page",
    "airbnb",
    "page",
  ];
  if (primary) {
    for (let k = 0; k < 10; k++) {
      const nights = 3 + (k % 3);
      const checkIn = addDays(base, -22 - k * 16);
      const checkOut = addDays(checkIn, nights);
      const source = rotation[k] as Source;
      const ota = ["airbnb", "booking"].includes(source);
      const q = ota
        ? { lines: [], total: 0 }
        : quote(p, { unitId: primary.id, from: checkIn, to: checkOut, adults: 2 });
      const names = [
        "Anna Svensson",
        "Kavindu Silva",
        "Lena Hoffmann",
        "James Wilson",
        "Ayesha Khan",
        "Marco Bianchi",
        "Sarah Miller",
      ];
      ref += 1;
      bookings.push({
        id: uid("bk"),
        ref: `${p.name
          .split(/\s+/)
          .map((w) => w[0]?.toUpperCase())
          .join("")
          .slice(0, 2)}-${ref - 40}`,
        propertyId: p.id,
        unitId: primary.id,
        source,
        status: "checked_out",
        guest: ota
          ? { name: source === "airbnb" ? "Airbnb stay" : "Booking.com stay" }
          : { name: names[k % names.length] as string },
        adults: 2,
        children: 0,
        checkIn,
        checkOut,
        lines: q.lines,
        total: q.total,
        payments: ota
          ? []
          : [
              {
                id: uid("pay"),
                amount: q.total,
                method: source === "whatsapp" ? "bank" : "card",
                at: addDays(checkIn, -10),
              },
            ],
        extras: [],
        createdAt: new Date(`${addDays(checkIn, -21)}T10:00:00`).toISOString(),
      });
    }
  }

  const blocks: Block[] = [
    {
      id: uid("blk"),
      propertyId: p.id,
      unitIds: [units[0]?.id ?? ""],
      from: addDays(base, 10),
      to: addDays(base, 11),
      reason: "maintenance",
      note: "Pool resurfacing",
    },
  ];

  const find = (name: string) => bookings.find((b) => b.guest.name === name);
  const unitName = (b?: Booking) => p.units.find((u) => u.id === b?.unitId)?.name ?? p.name;
  const activity: Activity[] = [];
  const add = (a: Omit<Activity, "id" | "propertyId">) =>
    activity.push({ id: uid("act"), propertyId: p.id, ...a });

  const priya = find("Priya Nair");
  if (priya)
    add({
      kind: "request",
      title: "Request from Priya Nair",
      subtitle: `${unitName(priya)}, 3 guests`,
      at: hoursAgo(1),
      read: false,
      bookingId: priya.id,
    });
  const lukas = find("Lukas Weber");
  if (lukas)
    add({
      kind: "request",
      title: "Request from Lukas Weber",
      subtitle: `${unitName(lukas)}, 3 nights`,
      at: hoursAgo(2),
      read: false,
      bookingId: lukas.id,
    });
  add({
    kind: "sync",
    title: "Airbnb sync failed",
    subtitle: `${p.name}, retrying every 15 minutes`,
    at: hoursAgo(2.3),
    read: false,
  });
  for (const name of ["Arjun Mehta", "Daniel Fernando"]) {
    const b = find(name);
    if (b?.slip)
      add({
        kind: "slip",
        title: `Bank slip from ${name}`,
        subtitle: `Balance for ${unitName(b)}`,
        at: daysAgo(1, 21, 12),
        read: false,
        bookingId: b.id,
      });
  }
  const bcom = bookings.find((b) => b.source === "booking");
  if (bcom)
    add({
      kind: "import",
      title: "Booking.com stay imported",
      subtitle: `${unitName(bcom)}, ${bcom.checkIn.slice(8)} to ${bcom.checkOut.slice(8)}`,
      at: daysAgo(1, 11, 20),
      read: true,
      bookingId: bcom.id,
    });
  const tom = find("Tom Becker");
  if (tom)
    add({
      kind: "cancellation",
      title: "Tom Becker cancelled",
      subtitle: `${unitName(tom)}, cancelled by the guest`,
      at: daysAgo(3, 16, 5),
      read: true,
      bookingId: tom.id,
    });
  const mia = find("Mia Schneider");
  if (mia)
    add({
      kind: "booking",
      title: "New direct booking from Mia Schneider",
      subtitle: `${unitName(mia)}, paid by card`,
      at: daysAgo(12, 9, 30),
      read: true,
      bookingId: mia.id,
    });
  const emma = find("Emma Larsen");
  if (emma)
    add({
      kind: "booking",
      title: "New direct booking from Emma Larsen",
      subtitle: `${unitName(emma)}, paid by card`,
      at: daysAgo(10, 9, 30),
      read: true,
      bookingId: emma.id,
    });

  activity.sort((a, b) => b.at.localeCompare(a.at));
  return { bookings, blocks, activity };
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

export function sampleTeam(
  owner: { name: string; phone: string },
  propertyId: string,
): TeamMember[] {
  return [
    {
      id: "tm_owner",
      name: owner.name || "You",
      phone: owner.phone,
      role: "owner",
      propertyIds: [],
      status: "active",
    },
    {
      id: uid("tm"),
      name: "Ruwan Silva",
      phone: "+94 77 222 3344",
      role: "manager",
      propertyIds: [],
      status: "active",
    },
    {
      id: uid("tm"),
      name: "Kamal Jayasinghe",
      phone: "+94 71 333 8899",
      role: "caretaker",
      propertyIds: [propertyId],
      status: "active",
    },
    {
      id: uid("tm"),
      name: "",
      phone: "+94 71 555 0199",
      role: "caretaker",
      propertyIds: [propertyId],
      status: "invited",
    },
  ];
}
