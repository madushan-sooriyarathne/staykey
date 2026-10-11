import type { ActivityPage } from "@staykey/api-client";
import { useInfiniteQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

export const activityKeys = {
  all: ["activity"] as const,
  list: (accountId: string) => ["activity", accountId] as const,
};

const fetchPage = (cursor: string | undefined) =>
  unwrap(api.GET("/v1/activity", { params: { query: { cursor, limit: 50 } } }));

function useActivityPages() {
  const accountId = useSession((s) => s.accountId);
  return useInfiniteQuery({
    queryKey: activityKeys.list(accountId),
    queryFn: ({ pageParam }) => fetchPage(pageParam),
    initialPageParam: undefined as string | undefined,
    getNextPageParam: (last: ActivityPage) => last.nextCursor,
    enabled: Boolean(accountId),
  });
}

/** The active account's activity, newest first, with paging for the list. */
export function useActivity() {
  const query = useActivityPages();
  const items = query.data?.pages.flatMap((p) => p.items) ?? [];
  return {
    items,
    loading: query.isPending,
    hasMore: query.hasNextPage,
    loadMore: () => {
      if (query.hasNextPage && !query.isFetchingNextPage) query.fetchNextPage();
    },
    refresh: query.refetch,
    refreshing: query.isRefetching,
  };
}

/** How many items the signed-in person hasn't read, from the first page's count. */
export function useUnreadCount(): number {
  return useActivityPages().data?.pages[0]?.unread ?? 0;
}

/** Marks everything read, showing it at once. */
export function useMarkActivityRead() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  const key = activityKeys.list(accountId);
  return useMutation({
    mutationFn: () => unwrap(api.POST("/v1/activity/read")),
    onMutate: async () => {
      await client.cancelQueries({ queryKey: key });
      client.setQueryData<{ pages: ActivityPage[]; pageParams: unknown[] }>(key, (data) =>
        data
          ? {
              ...data,
              pages: data.pages.map((p) => ({
                ...p,
                unread: 0,
                items: p.items.map((a) => ({ ...a, read: true })),
              })),
            }
          : data,
      );
    },
  });
}

/** Booking changes write activity on the server; this picks it up after one. */
export function useRefreshActivity() {
  const client = useQueryClient();
  return () => client.invalidateQueries({ queryKey: activityKeys.all });
}
