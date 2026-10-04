import { makeProperty } from "@/data/defaults";
import type { IcalFeed, PropertyConfig, Unit } from "@/data/types";
import { type Draft, OTA_CHANNELS } from "./store";

const minor = (major: string) => Math.round((Number(major) || 0) * 100);

/** Turns what the owner told us during onboarding into the property the app manages. */
export function propertyFromDraft(
  d: Draft,
  created: { id: string; slug: string; bookingPageUrl: string },
): PropertyConfig {
  const units: Unit[] =
    d.bookingType === "entire"
      ? [
          {
            id: `${created.id}_u1`,
            name: d.propertyName.trim(),
            sleeps: d.guests,
            beds: `${d.bedrooms} ${d.bedrooms === 1 ? "bedroom" : "bedrooms"}, ${d.beds} ${d.beds === 1 ? "bed" : "beds"}`,
            rate: minor(d.nightlyRate),
            weekendRate:
              d.weekendOn && Number(d.weekendRate) > 0 ? minor(d.weekendRate) : undefined,
          },
        ]
      : d.rooms.flatMap((r) =>
          Array.from({ length: r.count }, (_, i) => ({
            id: `${created.id}_${r.id}_${i + 1}`,
            name: r.count > 1 ? `${r.name.trim()} ${i + 1}` : r.name.trim(),
            sleeps: r.sleeps,
            beds: `Sleeps ${r.sleeps}`,
            rate: minor(r.rate),
          })),
        );

  const ical: IcalFeed[] = d.channels
    .filter((c) => OTA_CHANNELS.includes(c))
    .map((c) => ({
      id: `ical_${c}`,
      channel: c as IcalFeed["channel"],
      url: "",
      status: "pending",
      upcoming: 0,
    }));

  const houseRules = [
    d.noSmoking && "No smoking indoors",
    d.noParties && "No parties or events",
    d.petsAllowed ? "Pets are welcome" : "No pets",
  ].filter(Boolean) as string[];

  return makeProperty({
    id: created.id,
    name: d.propertyName.trim(),
    slug: created.slug,
    bookingPageUrl: created.bookingPageUrl,
    bookingType: d.bookingType,
    currency: d.currency,
    location: d.location.trim(),
    checkIn: d.checkIn,
    checkOut: d.checkOut,
    photos: d.photos.map((p) => ({ uri: p.uri })),
    units,
    policy: d.policy,
    depositPercent: d.depositPercent,
    houseRules,
    payments: {
      bank: {
        enabled: d.bankOn,
        bankName: d.bankName,
        accountName: d.accountName.trim(),
        accountNumber: d.accountNumber,
        payWithinHours: 24,
        cancelIfUnpaid: true,
      },
      atProperty: d.payAtProperty,
      cards: "off",
    },
    booking: {
      mode: ical.length ? "request" : "instant",
      replyHours: 24,
      holdMinutes: 15,
      displayCurrencies: [d.currency],
    },
    ical,
  });
}
