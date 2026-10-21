import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";
import { PrivacyNote } from "@/components/privacy-note";
import { getPublicProperty, quoteStay } from "@/lib/api";
import { formatRange, nightsBetween } from "@/lib/dates";
import { pagePath } from "@/lib/paths";
import { stayFromParams } from "@/lib/stay-params";
import { CheckoutForm } from "./checkout-form";

export const metadata: Metadata = { title: "Your booking | StayKey", robots: { index: false } };

export default async function BookPage(props: PageProps<"/p/[slug]/book">) {
  const { slug } = await props.params;
  const property = await getPublicProperty(slug);
  if (!property) notFound();
  const stay = stayFromParams(await props.searchParams);
  const unit = property.units.find((u) => u.id === stay.unitId);
  const back = await pagePath(slug);

  if (!unit || !stay.checkIn || !stay.checkOut || !property.takingBookings) {
    return (
      <main className="mx-auto flex max-w-xl flex-col gap-4 px-4 py-10">
        <h1 className="font-semibold text-3xl text-obsidian">Pick your dates first</h1>
        <Link href={back} className="text-ember-ink underline">
          Back to {property.name}
        </Link>
      </main>
    );
  }

  const adults = Math.min(Math.max(1, stay.adults ?? 2), unit.sleeps);
  const children = Math.min(stay.children ?? 0, unit.sleeps - adults);
  const priced = await quoteStay(slug, {
    unitId: unit.id,
    checkIn: stay.checkIn,
    checkOut: stay.checkOut,
    adults,
    children,
  });

  return (
    <main className="mx-auto flex max-w-xl flex-col gap-6 px-4 py-6 md:py-10">
      <Link href={back} className="text-sm text-steel hover:text-graphite">
        ‹ {property.name}
      </Link>
      <header className="flex flex-col gap-1">
        <h1 className="font-semibold text-3xl text-obsidian">Your stay</h1>
        <p className="text-graphite">
          {property.units.length > 1 && `${unit.name}, `}
          {formatRange(stay.checkIn, stay.checkOut)}, {nightsBetween(stay.checkIn, stay.checkOut)}{" "}
          nights
        </p>
        <p className="text-sm text-steel">
          Check-in from {property.checkIn}, check-out by {property.checkOut}
        </p>
      </header>
      <CheckoutForm
        slug={slug}
        back={back}
        stay={{ unitId: unit.id, checkIn: stay.checkIn, checkOut: stay.checkOut, adults, children }}
        sleeps={unit.sleeps}
        holdId={stay.hold}
        expiresAt={stay.expires}
        from={stay.from}
        currency={property.currency}
        extras={property.extras}
        payments={property.payments}
        mode={property.booking.mode}
        initialQuote={priced.ok ? { quote: priced.data } : { problem: priced.problem }}
      />
      <PrivacyNote propertyName={property.name} />
    </main>
  );
}
