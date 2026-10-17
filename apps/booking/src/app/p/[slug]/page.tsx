import { formatMoney, type PublicProperty } from "@staykey/api-client";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { BookingCard } from "@/components/booking-card";
import { getAvailability, getPublicProperty, quoteStay } from "@/lib/api";
import { addMonths, todayIn } from "@/lib/dates";
import { stayFromParams } from "@/lib/stay-params";
import type { QuoteState } from "./actions";

export async function generateMetadata(props: PageProps<"/p/[slug]">): Promise<Metadata> {
  const { slug } = await props.params;
  const property = await getPublicProperty(slug);
  if (!property) return { title: "Booking page not found | StayKey" };
  return {
    title: `${property.name} | Book direct`,
    description: `Book ${property.name}${property.location ? ` in ${property.location}` : ""} directly with the owner.`,
  };
}

// Keep in step with POLICY_TEXT in apps/mobile/src/data/pricing.ts.
const POLICIES: Record<PublicProperty["policy"], string> = {
  flexible: "Full refund up to 7 days before arrival.",
  moderate: "Full refund up to 14 days before arrival, then 50%.",
  strict: "50% refund up to 30 days before arrival, then none.",
};

export default async function PropertyPage(props: PageProps<"/p/[slug]">) {
  const { slug } = await props.params;
  const property = await getPublicProperty(slug);
  if (!property) notFound();

  const today = todayIn(property.timeZone);
  const initial = stayFromParams(await props.searchParams);
  const [availability, initialQuote] = await Promise.all([
    getAvailability(slug, today, addMonths(today, 12)),
    priceInitial(slug, property, initial),
  ]);
  const [cover, ...more] = property.photos;

  return (
    <main className="mx-auto flex max-w-5xl flex-col gap-8 px-4 py-6 md:px-6 md:py-10">
      <Photos cover={cover} more={more.slice(0, 4)} name={property.name} />

      <div className="grid gap-8 md:grid-cols-[1fr_400px]">
        <div className="flex flex-col gap-8">
          <section className="flex flex-col gap-3">
            <h1 className="font-semibold text-4xl text-obsidian md:text-5xl">{property.name}</h1>
            {property.location && <p className="text-steel">{property.location}</p>}
            <p className="text-graphite">
              {property.bookingType === "entire"
                ? "The whole place is yours."
                : "Book one room or more."}{" "}
              Rates start at {formatMoney(property.baseRate, property.currency)} a night.
            </p>
            {property.description && (
              <p className="whitespace-pre-line text-graphite">{property.description}</p>
            )}
          </section>

          {property.amenities.length > 0 && (
            <Section title="What's there">
              <ul className="flex flex-wrap gap-2">
                {property.amenities.map((a) => (
                  <li key={a} className="rounded-badge bg-cloud px-3 py-1 text-graphite text-sm">
                    {a}
                  </li>
                ))}
              </ul>
            </Section>
          )}

          <Section title={property.units.length > 1 ? "Rooms" : "Sleeping"}>
            <ul className="flex flex-col gap-2">
              {property.units.map((u) => (
                <li key={u.id} className="flex justify-between gap-4 text-graphite">
                  <span>{u.name}</span>
                  <span className="text-steel">
                    Sleeps {u.sleeps}
                    {u.beds && `, ${u.beds}`}
                  </span>
                </li>
              ))}
            </ul>
          </Section>

          <Section title="Good to know">
            <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-2 text-graphite">
              <dt className="text-steel">Check-in</dt>
              <dd>From {property.checkIn}</dd>
              <dt className="text-steel">Check-out</dt>
              <dd>By {property.checkOut}</dd>
              <dt className="text-steel">Cancelling</dt>
              <dd>{POLICIES[property.policy]}</dd>
            </dl>
            {property.houseRules.length > 0 && (
              <ul className="mt-3 list-disc pl-5 text-graphite">
                {property.houseRules.map((r) => (
                  <li key={r}>{r}</li>
                ))}
              </ul>
            )}
          </Section>
        </div>

        <aside className="md:sticky md:top-6 md:self-start">
          {property.takingBookings ? (
            <BookingCard
              slug={slug}
              baseRate={property.baseRate}
              currency={property.currency}
              units={property.units}
              availability={availability.units}
              rules={property.rules}
              today={today}
              initial={initial}
              initialQuote={initialQuote}
              from={initial.from}
            />
          ) : (
            <div className="rounded-card border border-cloud bg-snow p-5 text-graphite">
              {property.name} isn't taking bookings online right now. Please contact the property
              directly.
            </div>
          )}
          <p className="mt-3 text-center text-fog text-sm">
            Book direct with the owner. No booking fees.
          </p>
        </aside>
      </div>
    </main>
  );
}

/** Prices the stay a link arrived with, so the card shows it from the start. */
async function priceInitial(
  slug: string,
  property: PublicProperty,
  stay: ReturnType<typeof stayFromParams>,
): Promise<QuoteState> {
  const unitId = property.units.some((u) => u.id === stay.unitId)
    ? stay.unitId
    : property.units[0]?.id;
  if (!unitId || !stay.checkIn || !stay.checkOut) return {};
  const res = await quoteStay(slug, {
    unitId,
    checkIn: stay.checkIn,
    checkOut: stay.checkOut,
    adults: Math.max(1, stay.adults ?? 2),
    children: stay.children ?? 0,
  });
  return res.ok ? { quote: res.data } : { problem: res.problem };
}

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <section className="flex flex-col gap-3 border-cloud border-t pt-6">
      <h2 className="font-semibold text-obsidian text-xl">{title}</h2>
      {children}
    </section>
  );
}

type Photo = PublicProperty["photos"][number];

function Photos({ cover, more, name }: { cover?: Photo; more: Photo[]; name: string }) {
  if (!cover) {
    return (
      <div className="flex h-64 items-end rounded-card-lg bg-gradient-to-br from-cloud to-mist p-6 text-sm text-steel md:h-96">
        {name}
      </div>
    );
  }
  // Photos come from the owner's uploads on the API's file store, whose host differs per
  // environment, so they are plain images rather than next/image.
  return (
    <div className="grid h-64 gap-2 overflow-hidden rounded-card-lg md:h-96 md:grid-cols-[2fr_1fr]">
      {/* biome-ignore lint/performance/noImgElement: see the note above */}
      <img
        src={cover.url}
        alt={cover.caption || name}
        width={1200}
        height={800}
        className="h-full w-full object-cover"
      />
      {more.length > 0 && (
        <div className="hidden grid-cols-2 gap-2 md:grid">
          {more.map((p) => (
            // biome-ignore lint/performance/noImgElement: see the note above
            <img
              key={p.url}
              src={p.url}
              alt={p.caption || name}
              width={600}
              height={400}
              loading="lazy"
              className="h-full w-full object-cover"
            />
          ))}
        </div>
      )}
    </div>
  );
}
