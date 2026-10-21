import type { PayoutProfileInput } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { cardKeys } from "./cards";
import { api } from "./client";
import { unwrap } from "./errors";

export const payoutKeys = {
  list: (accountId: string) => ["payouts", accountId] as const,
  detail: (accountId: string, id: string) => ["payouts", accountId, id] as const,
};

/**
 * Saves the payout details. The card setup carries them and whether payouts can be chosen, so
 * it is read again.
 */
export function useSetPayoutProfile() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: (body: PayoutProfileInput) =>
      unwrap(api.PUT("/v1/card-payments/payout-profile", { body })),
    onSuccess: () =>
      Promise.all([
        client.invalidateQueries({ queryKey: cardKeys.detail(accountId) }),
        client.invalidateQueries({ queryKey: payoutKeys.list(accountId) }),
      ]),
  });
}

/** Payout balances, the next payout day and past payouts. */
export function usePayouts() {
  const accountId = useSession((s) => s.accountId);
  return useQuery({
    queryKey: payoutKeys.list(accountId),
    queryFn: () => unwrap(api.GET("/v1/payouts")),
    enabled: Boolean(accountId),
  });
}

/** One payout with the entries it paid. */
export function usePayout(id: string) {
  const accountId = useSession((s) => s.accountId);
  return useQuery({
    queryKey: payoutKeys.detail(accountId, id),
    queryFn: () =>
      unwrap(api.GET("/v1/payouts/{payoutId}", { params: { path: { payoutId: id } } })),
    enabled: Boolean(accountId && id),
  });
}
