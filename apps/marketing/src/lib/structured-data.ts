import type { Plan } from "@staykey/api-client";
import { COMPANY, SITE_URL, SUPPORT } from "./company";

/**
 * The home page's JSON-LD: the organisation and the app, with the web prices as offers when
 * they loaded. Escaped for a script tag, as the Next.js JSON-LD guide recommends.
 */
export function structuredData(plans: Plan[] | null): string {
  const graph: Record<string, unknown>[] = [
    {
      "@type": "Organization",
      "@id": `${SITE_URL}/#organization`,
      name: "StayKey",
      legalName: COMPANY.name,
      url: SITE_URL,
      logo: `${SITE_URL}/icon.png`,
      email: SUPPORT.email,
      address: {
        "@type": "PostalAddress",
        streetAddress: "No. 18, Gnanathilake Road",
        addressLocality: "Mount Lavinia",
        addressCountry: "LK",
      },
      contactPoint: {
        "@type": "ContactPoint",
        contactType: "customer support",
        email: SUPPORT.email,
        telephone: SUPPORT.whatsapp.replace(/\s/g, ""),
        availableLanguage: "English",
      },
    },
    {
      "@type": "SoftwareApplication",
      name: "StayKey",
      url: SITE_URL,
      description:
        "A booking page, website widget and one calendar for villa and guesthouse owners in Sri Lanka, set up from your phone.",
      applicationCategory: "BusinessApplication",
      operatingSystem: "iOS, Android",
      publisher: { "@id": `${SITE_URL}/#organization` },
      ...(plans
        ? {
            offers: plans.map((p) => ({
              "@type": "Offer",
              name: `${p.name} plan, monthly`,
              price: (p.monthly / 100).toFixed(2),
              priceCurrency: p.currency,
              url: `${SITE_URL}/pricing`,
            })),
          }
        : {}),
    },
  ];
  return JSON.stringify({ "@context": "https://schema.org", "@graph": graph }).replace(
    /</g,
    "\\u003c",
  );
}
