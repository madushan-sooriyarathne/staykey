import type { Calendar, NewBlock, RateOverrideInput } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { addDays, eachNight, today } from "@/data/dates";
import { toCalendar } from "@/data/from-api";
import type { Overrides } from "@/data/pricing";
import type { Block } from "@/data/types";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

/** Keys include the account and property, so each property's calendar caches on its own. */
export const calendarKeys = {
  all: ["calendar"] as const,
  property: (accountId: string, propertyId: string) => ["calendar", accountId, propertyId] as const,
};

/** The calendar loads two months back and two years ahead, the span the API allows. */
const range = () => ({ from: addDays(today(), -60), to: addDays(today(), 730) });

const fetchCalendar = (id: string) =>
  unwrap(api.GET("/v1/properties/{id}/calendar", { params: { path: { id }, query: range() } }));

const EMPTY = { blocks: [] as Block[], overrides: {} as Overrides };

/** A property's blocks and rate overrides. */
export function useCalendar(propertyId?: string): { blocks: Block[]; overrides: Overrides } {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: calendarKeys.property(accountId, propertyId ?? ""),
    queryFn: () => fetchCalendar(propertyId ?? ""),
    enabled: Boolean(accountId && propertyId),
    select: toCalendar,
  });
  return data ?? EMPTY;
}

/** Changes a property's cached calendar straight away; the server's copy follows. */
function useCalendarMutation<Vars extends { propertyId: string }>(
  send: (vars: Vars) => Promise<unknown>,
  guess: (c: Calendar, vars: Vars) => Calendar,
) {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: send,
    onMutate: async (vars: Vars) => {
      const key = calendarKeys.property(accountId, vars.propertyId);
      await client.cancelQueries({ queryKey: key });
      const before = client.getQueryData<Calendar>(key);
      if (before) client.setQueryData(key, guess(before, vars));
      return { before };
    },
    onError: (_error, vars, context) => {
      if (context?.before)
        client.setQueryData(calendarKeys.property(accountId, vars.propertyId), context.before);
    },
    onSettled: (_data, _error, vars) =>
      client.invalidateQueries({ queryKey: calendarKeys.property(accountId, vars.propertyId) }),
  });
}

/** Blocks nights. The server refuses nights a stay holds. */
export function useCreateBlock() {
  return useCalendarMutation(
    (body: NewBlock) => unwrap(api.POST("/v1/blocks", { body })),
    (c, b) => ({
      ...c,
      blocks: [...c.blocks, { ...b, id: "pending", note: b.note ?? "" }],
    }),
  );
}

export function useDeleteBlock() {
  return useCalendarMutation(
    ({ id }: { id: string; propertyId: string }) =>
      unwrap(api.DELETE("/v1/blocks/{blockId}", { params: { path: { blockId: id } } })),
    (c, { id }) => ({ ...c, blocks: c.blocks.filter((b) => b.id !== id) }),
  );
}

/** Sets or clears the price and stay rules for some nights on some units. */
export function useSetRateOverrides() {
  return useCalendarMutation(
    (body: RateOverrideInput) => unwrap(api.PUT("/v1/rate-overrides", { body })),
    (c, o) => {
      const nights = eachNight(o.from, o.to);
      const kept = c.rateOverrides.filter(
        (r) => !(o.unitIds.includes(r.unitId) && nights.includes(r.night)),
      );
      const set = o.price != null || o.minNights != null || o.closedToArrival;
      const added = set
        ? o.unitIds.flatMap((unitId) =>
            nights.map((night) => ({
              unitId,
              night,
              price: o.price,
              minNights: o.minNights,
              closedToArrival: o.closedToArrival ?? false,
            })),
          )
        : [];
      return { ...c, rateOverrides: [...kept, ...added] };
    },
  );
}
