import AsyncStorage from "@react-native-async-storage/async-storage";
import { createAsyncStoragePersister } from "@tanstack/query-async-storage-persister";
import { QueryClient } from "@tanstack/react-query";
import { setOnSessionEnded } from "./client";
import { ApiError } from "./errors";

const WEEK = 7 * 24 * 60 * 60 * 1000;

/**
 * Server data cache for the app. Client errors (4xx) are not retried, since asking again gives
 * the same answer; network and server errors retry twice. Data is kept for a week and saved to
 * the device, so properties open instantly and stay readable offline.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      gcTime: WEEK,
      retry: (failures, error) =>
        !(error instanceof ApiError && error.status < 500) && failures < 2,
    },
    mutations: { retry: false },
  },
});

export const persister = createAsyncStoragePersister({
  storage: AsyncStorage,
  key: "staykey.queries",
  throttleTime: 1000,
});

/** Bump buster when cached shapes change, so old caches are dropped instead of misread. */
export const persistOptions = { persister, maxAge: WEEK, buster: "phase-2" };

/** Forgets every cached response, on this device too. */
export async function clearCache() {
  queryClient.clear();
  await persister.removeClient();
}

// Nothing cached for one session should be visible to the next.
setOnSessionEnded(() => {
  clearCache();
});
