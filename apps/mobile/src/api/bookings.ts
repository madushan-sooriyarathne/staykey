import type {
  Booking as ApiBooking,
  BookingPatch,
  Cancellation,
  NewBooking,
  NewPayment,
  Transition,
} from "@staykey/api-client";
import { type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { addDays, today } from "@/data/dates";
import { toBooking } from "@/data/from-api";
import type { Booking } from "@/data/types";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

/** Keys include the account, so switching accounts never shows another account's stays. */
export const bookingKeys = {
  all: ["bookings"] as const,
  list: (accountId: string) => ["bookings", accountId] as const,
};

/** How far back stays load: a year and a bit, enough for Insights to compare periods. */
const HISTORY_DAYS = 400;

/** Every stay the caller can see from HISTORY_DAYS ago on, page by page. */
export async function fetchBookings(): Promise<ApiBooking[]> {
  const from = addDays(today(), -HISTORY_DAYS);
  const items: ApiBooking[] = [];
  let cursor: string | undefined;
  do {
    const page = await unwrap(
      api.GET("/v1/bookings", { params: { query: { from, limit: 500, cursor } } }),
    );
    items.push(...page.items);
    cursor = page.nextCursor;
  } while (cursor);
  return items;
}

const EMPTY: Booking[] = [];
// Module-level so TanStack Query keeps the mapped array stable between renders.
const toBookings = (items: ApiBooking[]) => items.map(toBooking);

/** The active account's stays, as the screens' model. */
export function useBookings(): Booking[] {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: bookingKeys.list(accountId),
    queryFn: fetchBookings,
    enabled: Boolean(accountId),
    select: toBookings,
  });
  return data ?? EMPTY;
}

export const useBooking = (id?: string) => useBookings().find((b) => b.id === id);

/** Puts the server's copy of a stay into the cached list. */
function storeBooking(client: QueryClient, accountId: string, booking: ApiBooking) {
  client.setQueryData<ApiBooking[]>(bookingKeys.list(accountId), (items = []) =>
    items.some((b) => b.id === booking.id)
      ? items.map((b) => (b.id === booking.id ? booking : b))
      : [...items, booking],
  );
}

/**
 * Changes a cached stay straight away and puts it back if the server says no. The server's
 * answer replaces the guess either way.
 */
function useOptimistic<Vars extends { id: string }>(
  send: (vars: Vars) => Promise<ApiBooking>,
  guess: (b: ApiBooking, vars: Vars) => ApiBooking,
) {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  const key = bookingKeys.list(accountId);
  return useMutation({
    mutationFn: send,
    onMutate: async (vars: Vars) => {
      await client.cancelQueries({ queryKey: key });
      const before = client.getQueryData<ApiBooking[]>(key);
      client.setQueryData<ApiBooking[]>(key, (items = []) =>
        items.map((b) => (b.id === vars.id ? guess(b, vars) : b)),
      );
      return { before };
    },
    onError: (_error, _vars, context) => {
      if (context?.before) client.setQueryData(key, context.before);
    },
    onSuccess: (booking) => storeBooking(client, accountId, booking),
  });
}

/** Adds a stay. Waits for the server, which prices it and checks the nights are free. */
export function useCreateBooking() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: ({ body, key }: { body: NewBooking; key: string }) =>
      unwrap(api.POST("/v1/bookings", { body, params: { header: { "Idempotency-Key": key } } })),
    onSuccess: (booking) => storeBooking(client, accountId, booking),
  });
}

/** Saves an edit. Waits for the server, which prices the stay again when that changed. */
export function useUpdateBooking() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: BookingPatch }) =>
      unwrap(
        api.PATCH("/v1/bookings/{bookingId}", { params: { path: { bookingId: id } }, body: patch }),
      ),
    onSuccess: (booking) => storeBooking(client, accountId, booking),
  });
}

/** Approves, declines, confirms, checks in or checks out, showing the change at once. */
export function useTransition() {
  return useOptimistic(
    ({ id, ...body }: Transition & { id: string }) =>
      unwrap(
        api.POST("/v1/bookings/{bookingId}/transitions", {
          params: { path: { bookingId: id } },
          body,
        }),
      ),
    (b, { to }) => ({ ...b, status: to }),
  );
}

/** Records a payment, showing it at once. A first payment confirms a stay waiting for one. */
export function useRecordPayment() {
  return useOptimistic(
    ({ id, key, ...body }: NewPayment & { id: string; key: string }) =>
      unwrap(
        api.POST("/v1/bookings/{bookingId}/payments", {
          params: { path: { bookingId: id }, header: { "Idempotency-Key": key } },
          body,
        }),
      ),
    (b, { amount, method, slipId }) => ({
      ...b,
      status: b.status === "awaiting_payment" ? "confirmed" : b.status,
      payments: [
        ...(b.payments ?? []),
        {
          id: "pending",
          kind: "payment",
          method,
          amount,
          note: "",
          receivedAt: new Date().toISOString(),
        },
      ],
      slips: b.slips?.map((s) => (s.id === slipId ? { ...s, status: "accepted" } : s)),
    }),
  );
}

/** Turns down a guest's bank slip, showing it at once. */
export function useRejectSlip() {
  return useOptimistic(
    ({ slipId }: { id: string; slipId: string }) =>
      unwrap(api.POST("/v1/slips/{slipId}/reject", { params: { path: { slipId } } })),
    (b, { slipId }) => ({
      ...b,
      slips: b.slips?.map((s) => (s.id === slipId ? { ...s, status: "rejected" } : s)),
    }),
  );
}

/** Cancels a stay with the refund the owner chose. Waits for the server, since money moves. */
export function useCancelBooking() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: ({ id, key, ...body }: Cancellation & { id: string; key: string }) =>
      unwrap(
        api.POST("/v1/bookings/{bookingId}/cancel", {
          params: { path: { bookingId: id }, header: { "Idempotency-Key": key } },
          body,
        }),
      ),
    onSuccess: (booking) => storeBooking(client, accountId, booking),
  });
}
