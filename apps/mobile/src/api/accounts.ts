import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";
import { meKeys } from "./me";

export const accountKeys = {
  all: ["accounts"] as const,
  list: (userId: string) => ["accounts", userId] as const,
};

/** Accounts the signed-in user belongs to, with their role in each. */
export function useAccounts() {
  const userId = useSession((s) => s.userId);
  return useQuery({
    queryKey: accountKeys.list(userId),
    queryFn: async () => (await unwrap(api.GET("/v1/accounts"))).items,
    enabled: Boolean(userId),
  });
}

export const createAccount = (name: string) => unwrap(api.POST("/v1/accounts", { body: { name } }));

export function useCreateAccount() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: createAccount,
    onSuccess: () => {
      client.invalidateQueries({ queryKey: accountKeys.all });
      client.invalidateQueries({ queryKey: meKeys.all });
    },
  });
}

/** Renames the active account. Owners only. */
export function useRenameAccount() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: (name: string) => unwrap(api.PATCH("/v1/account", { body: { name } })),
    onSuccess: () => {
      client.invalidateQueries({ queryKey: accountKeys.all });
      client.invalidateQueries({ queryKey: meKeys.all });
    },
  });
}

/** Everything in the active account as one document. Owners only. */
export const exportAccount = () => unwrap(api.GET("/v1/account/export"));
