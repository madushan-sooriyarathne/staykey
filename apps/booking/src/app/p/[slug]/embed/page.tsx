import { notFound } from "next/navigation";
import { getPublicProperty } from "@/lib/api";
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

  return (
    <EmbedBridge
      baseRate={property.baseRate}
      currency={property.currency}
      bookingPageUrl={bookingPageUrl(slug)}
      hostOrigin={parseOrigin(host)}
    />
  );
}
