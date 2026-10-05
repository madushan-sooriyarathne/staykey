import { ApiError, messageFor } from "@/api/errors";
import { createProperty, storeProperty } from "@/api/properties";
import { queryClient } from "@/api/query-client";
import { toNewProperty, toPropertyConfig } from "@/data/from-api";
import { sampleGuesthouse } from "@/data/seed";
import { useData } from "@/data/store";
import { useSession } from "@/lib/session";

/**
 * Prototype tool: publishes the sample guesthouse (rooms, a whole-house unit, seasons, extras and
 * promo codes) to the signed-in account, then adds sample alerts for it on this device. Sample
 * stays come from cmd/seed on the server. Returns an error message, or null when it worked.
 */
export async function addSampleGuesthouse(): Promise<string | null> {
  const house = sampleGuesthouse();
  try {
    for (let attempt = 0; attempt < 4; attempt++) {
      const slug =
        attempt === 0 ? "coralbay" : `coralbay-${Math.random().toString(36).slice(2, 6)}`;
      try {
        const created = await createProperty(toNewProperty({ ...house, slug, photos: [] }));
        storeProperty(queryClient, useSession.getState().accountId, created);
        useData.getState().addSample(toPropertyConfig(created), true);
        return null;
      } catch (e) {
        if (!(e instanceof ApiError && e.code === "slug_taken")) throw e;
      }
    }
    return "Couldn't find a free booking page address for the sample.";
  } catch (e) {
    return messageFor(e);
  }
}
