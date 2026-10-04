import { QueryClient } from "@tanstack/react-query";
import { setOnSessionEnded } from "./client";
import { ApiError } from "./errors";

/**
 * Server data cache for the app. Client errors (4xx) are not retried, since asking again gives
 * the same answer; network and server errors retry twice.
 */
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: (failures, error) =>
        !(error instanceof ApiError && error.status < 500) && failures < 2,
    },
    mutations: { retry: false },
  },
});

// Nothing cached for one session should be visible to the next.
setOnSessionEnded(() => queryClient.clear());
