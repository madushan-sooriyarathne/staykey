import type { NotificationPref, Template } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

export const templateKeys = {
  all: ["templates"] as const,
  list: (accountId: string) => ["templates", accountId] as const,
};

export const prefKeys = {
  all: ["notification-prefs"] as const,
  list: (accountId: string) => ["notification-prefs", accountId] as const,
};

const NO_TEMPLATES: Template[] = [];
const NO_PREFS: NotificationPref[] = [];

/** The active account's message templates. */
export function useTemplates(): Template[] {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: templateKeys.list(accountId),
    queryFn: async () => (await unwrap(api.GET("/v1/templates"))).items,
    enabled: Boolean(accountId),
  });
  return data ?? NO_TEMPLATES;
}

export function useUpdateTemplate() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: ({ id, body }: { id: string; body: string }) =>
      unwrap(
        api.PATCH("/v1/templates/{templateId}", {
          params: { path: { templateId: id } },
          body: { body },
        }),
      ),
    onSuccess: (saved) =>
      client.setQueryData<Template[]>(templateKeys.list(accountId), (list = []) =>
        list.map((t) => (t.id === saved.id ? saved : t)),
      ),
  });
}

/** How the signed-in person wants to hear about each event in the active account. */
export function useNotificationPrefs(): NotificationPref[] {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: prefKeys.list(accountId),
    queryFn: async () => (await unwrap(api.GET("/v1/notification-prefs"))).items,
    enabled: Boolean(accountId),
  });
  return data ?? NO_PREFS;
}

/** Saves one event's preference, showing the switch flip at once. */
export function useSetNotificationPref() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  const key = prefKeys.list(accountId);
  return useMutation({
    mutationFn: async (pref: NotificationPref) =>
      (await unwrap(api.PUT("/v1/notification-prefs", { body: { items: [pref] } }))).items,
    onMutate: async (pref) => {
      await client.cancelQueries({ queryKey: key });
      const before = client.getQueryData<NotificationPref[]>(key);
      client.setQueryData<NotificationPref[]>(key, (list = []) =>
        list.map((p) => (p.event === pref.event ? pref : p)),
      );
      return { before };
    },
    onError: (_error, _pref, context) => {
      if (context?.before) client.setQueryData(key, context.before);
    },
    onSuccess: (items) => client.setQueryData(key, items),
  });
}
