import type { CardMode, CardPayments } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";
import { propertyKeys } from "./properties";

export const cardKeys = {
  detail: (accountId: string) => ["card-payments", accountId] as const,
};

/** Where the account's guest card payments go. Owners only. */
export function useCardPayments(options?: { enabled?: boolean }) {
  const accountId = useSession((s) => s.accountId);
  return useQuery({
    queryKey: cardKeys.detail(accountId),
    queryFn: () => unwrap(api.GET("/v1/card-payments")),
    enabled: Boolean(accountId) && options?.enabled !== false,
  });
}

/**
 * Saves a change to the card setup. Properties are read again, since whether their cards work
 * follows the account's mode and connection.
 */
function useCardChange<T>(send: (input: T) => Promise<CardPayments>) {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: send,
    onSuccess: (data: CardPayments) => {
      client.setQueryData(cardKeys.detail(accountId), data);
      return client.invalidateQueries({ queryKey: propertyKeys.all });
    },
  });
}

export function useSetCardMode() {
  return useCardChange((mode: CardMode) =>
    unwrap(api.PUT("/v1/card-payments/mode", { body: { mode } })),
  );
}

export function useConnectCards() {
  return useCardChange((secretKey: string) =>
    unwrap(api.PUT("/v1/card-payments/connection", { body: { secretKey } })),
  );
}

export function useDisconnectCards() {
  return useCardChange(() => unwrap(api.DELETE("/v1/card-payments/connection")));
}
