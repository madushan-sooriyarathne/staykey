import { formatMoney } from "@staykey/api-client";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { BookingCard } from "@/components/booking-card";
import { getPublicProperty } from "@/lib/api";

export async function generateMetadata(props: PageProps<"/p/[slug]">): Promise<Metadata> {
  const { slug } = await props.params;
  const property = await getPublicProperty(slug);
  if (!property) return { title: "Booking page not found | StayKey" };
  return {
    title: `${property.name} | Book direct`,
    description: `Book ${property.name}${property.location ? ` in ${property.location}` : ""} directly with the owner.`,
  };
}

export default async function PropertyPage(props: PageProps<"/p/[slug]">) {
  const { slug } = await props.params;
  const property = await getPublicProperty(slug);
  if (!property) notFound();

  return (
    <main className="mx-auto flex max-w-5xl flex-col gap-8 px-4 py-6 md:px-6 md:py-10">
      <div className="flex h-64 items-end rounded-card-lg bg-gradient-to-br from-cloud to-mist p-6 text-sm text-steel md:h-96">
        Cover photo
      </div>

      <div className="grid gap-8 md:grid-cols-[1fr_360px]">
        <section className="flex flex-col gap-3">
          <h1 className="font-semibold text-4xl text-obsidian md:text-5xl">{property.name}</h1>
          {property.location && <p className="text-steel">{property.location}</p>}
          <p className="text-graphite">
            {property.bookingType === "entire"
              ? "The whole place is yours."
              : "Book one room or more."}{" "}
            Rates start at {formatMoney(property.baseRate, property.currency)} a night.
          </p>
        </section>

        <aside className="md:sticky md:top-6 md:self-start">
          <BookingCard baseRate={property.baseRate} currency={property.currency} />
          <p className="mt-3 text-center text-fog text-sm">
            Book direct with the owner. No booking fees.
          </p>
        </aside>
      </div>
    </main>
  );
}
