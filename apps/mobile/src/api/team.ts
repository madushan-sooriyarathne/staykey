import type { MemberPatch, NewInvite, Team } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { accountKeys } from "./accounts";
import { api } from "./client";
import { unwrap } from "./errors";
import { meKeys } from "./me";

export const teamKeys = {
  all: ["team"] as const,
  detail: (accountId: string) => ["team", accountId] as const,
};

const EMPTY: Team = { members: [], invites: [] };

/** The active account's members and open invites. */
export function useTeam(): Team {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: teamKeys.detail(accountId),
    queryFn: () => unwrap(api.GET("/v1/team")),
    enabled: Boolean(accountId),
  });
  return data ?? EMPTY;
}

function useInvalidateTeam() {
  const client = useQueryClient();
  return () => client.invalidateQueries({ queryKey: teamKeys.all });
}

/** Invites someone and returns the link to send them. The link is never shown again. */
export function useInvite() {
  const invalidate = useInvalidateTeam();
  return useMutation({
    mutationFn: (body: NewInvite) => unwrap(api.POST("/v1/team/invites", { body })),
    onSuccess: invalidate,
  });
}

export function useRevokeInvite() {
  const invalidate = useInvalidateTeam();
  return useMutation({
    mutationFn: (inviteId: string) =>
      unwrap(api.DELETE("/v1/team/invites/{inviteId}", { params: { path: { inviteId } } })),
    onSuccess: invalidate,
  });
}

export function useUpdateMember() {
  const invalidate = useInvalidateTeam();
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: MemberPatch }) =>
      unwrap(
        api.PATCH("/v1/team/members/{membershipId}", {
          params: { path: { membershipId: id } },
          body: patch,
        }),
      ),
    onSuccess: invalidate,
  });
}

export function useRemoveMember() {
  const invalidate = useInvalidateTeam();
  return useMutation({
    mutationFn: (id: string) =>
      unwrap(
        api.DELETE("/v1/team/members/{membershipId}", { params: { path: { membershipId: id } } }),
      ),
    onSuccess: invalidate,
  });
}

/** What an invite link is for, before the person signs in. */
export const previewInvite = (token: string) =>
  unwrap(api.POST("/v1/public/invites/preview", { body: { token } }));

export function useInvitePreview(token: string | undefined) {
  return useQuery({
    queryKey: ["invite", token],
    queryFn: () => previewInvite(token ?? ""),
    enabled: Boolean(token),
    gcTime: 0,
  });
}

/** Joins the account an invite is for. The signed-in number must be the invited one. */
export async function acceptInvite(token: string) {
  const account = await unwrap(api.POST("/v1/invites/accept", { body: { token } }));
  return account;
}

/** Refreshes the lists that change when someone joins an account. */
export function useAfterJoin() {
  const client = useQueryClient();
  return () => {
    client.invalidateQueries({ queryKey: accountKeys.all });
    client.invalidateQueries({ queryKey: meKeys.all });
  };
}
