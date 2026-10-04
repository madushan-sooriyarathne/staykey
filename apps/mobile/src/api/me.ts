import type { UserPatch } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

export const meKeys = {
  all: ["me"] as const,
  detail: (userId: string) => ["me", userId] as const,
};

export const fetchMe = () => unwrap(api.GET("/v1/me"));

/** The signed-in user and their accounts. */
export function useMe(options?: { enabled?: boolean }) {
  const userId = useSession((s) => s.userId);
  return useQuery({
    queryKey: meKeys.detail(userId),
    queryFn: fetchMe,
    enabled: Boolean(userId) && options?.enabled !== false,
  });
}

export const updateMe = (patch: UserPatch) => unwrap(api.PATCH("/v1/me", { body: patch }));

export function useUpdateMe() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: updateMe,
    onSuccess: () => client.invalidateQueries({ queryKey: meKeys.all }),
  });
}
