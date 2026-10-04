import "server-only";
import { createStayKeyClient, type PublicProperty } from "@staykey/api-client";

const api = createStayKeyClient(process.env.STAYKEY_API_URL ?? "http://localhost:8080");

/** Loads a property's public booking page details, or null when the slug is unknown. */
export async function getPublicProperty(slug: string): Promise<PublicProperty | null> {
  const { data, response } = await api.GET("/v1/public/properties/{slug}", {
    params: { path: { slug } },
  });
  if (response.status === 404) return null;
  if (!data) throw new Error(`StayKey API returned ${response.status} for property "${slug}"`);
  return data;
}
