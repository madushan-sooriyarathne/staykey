import { notFound } from "next/navigation";
import { getAvailability, getPublicProperty } from "@/lib/api";
import { addMonths, todayIn } from "@/lib/dates";
import { bookingPageUrl } from "@/lib/tenant";
import { EmbedBridge } from "./embed-bridge";

/** Only accept a well-formed http(s) origin for postMessage targeting. */
function parseOrigin(value: string | string[] | undefined): string | null {
  if (typeof value !== "string") return null;
  try {
    const url = new URL(value);
    return url.protocol === "https:" || url.protocol === "http:" ? url.origin : null;
  } catch {
    return null;
  }
}

export default async function EmbedPage(props: PageProps<"/p/[slug]/embed">) {
  const { slug } = await props.params;
  const { host } = await props.searchParams;
  const property = await getPublicProperty(slug);
  if (!property) notFound();
  const today = todayIn(property.timeZone);
  const availability = await getAvailability(slug, today, addMonths(today, 12));

  return (
    <EmbedBridge
      slug={slug}
      baseRate={property.baseRate}
      currency={property.currency}
      units={property.units}
      availability={availability.units}
      rules={property.rules}
      today={today}
      takingBookings={property.takingBookings}
      bookingPageUrl={bookingPageUrl(slug)}
      hostOrigin={parseOrigin(host)}
    />
  );
}
