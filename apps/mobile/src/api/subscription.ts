import type { Subscription } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

export const subscriptionKeys = {
  detail: (accountId: string) => ["subscription", accountId] as const,
};

/**
 * The account's plan as the server sees it. The server decides access from RevenueCat and web
 * payments, so screens read this, never the store.
 */
export function useSubscription() {
  const accountId = useSession((s) => s.accountId);
  return useQuery({
    queryKey: subscriptionKeys.detail(accountId),
    queryFn: () => unwrap(api.GET("/v1/subscription")),
    enabled: Boolean(accountId),
  });
}

/** Asks the server to read the store plans now, after a purchase or a restore. */
export function useRefreshSubscription() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: () => unwrap(api.POST("/v1/subscription/refresh")),
    onSuccess: (data: Subscription) =>
      client.setQueryData(subscriptionKeys.detail(accountId), data),
  });
}
