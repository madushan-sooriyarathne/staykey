import { create } from "zustand";
import { persist } from "zustand/middleware";
import { persistentStorage } from "@/lib/storage";
import { formatRange, today } from "./dates";
import { DEFAULT_ACCOUNT, DEFAULT_NOTIFICATIONS, DEFAULT_TEMPLATES, uid } from "./defaults";
import { nextRef, type Overrides } from "./pricing";
import { sampleData, sampleTeam } from "./seed";
import type {
  Account,
  Activity,
  Block,
  Booking,
  BookingStatus,
  ISODate,
  NotificationEvent,
  Payment,
  PropertyConfig,
  RateOverride,
  TeamMember,
  Template,
} from "./types";

/**
 * Data the API doesn't hold yet: bookings, blocks, rate overrides, activity, team, templates and
 * notification settings. Properties and their settings come from the API (src/api/properties).
 */
type DataState = {
  bookings: Booking[];
  blocks: Block[];
  /** Per property, per unit, per night. */
  overrides: Record<string, Overrides>;
  activity: Activity[];
  team: TeamMember[];
  templates: Template[];
  notifications: Record<NotificationEvent, { push: boolean; whatsapp: boolean }>;
  account: Account;

  /** After publishing or signing in: the owner's details, plus sample stays when asked. */
  startLocal: (
    p: PropertyConfig | undefined,
    account: Partial<Account>,
    withSample: boolean,
  ) => void;
  /** Prototype only: sample stays for another property, such as the sample guesthouse. */
  addSample: (p: PropertyConfig, alt?: boolean) => void;

  createBooking: (
    b: Omit<Booking, "id" | "ref" | "createdAt">,
    property?: PropertyConfig,
  ) => Booking;
  updateBooking: (id: string, patch: Partial<Booking>) => void;
  setStatus: (id: string, status: BookingStatus) => void;
  recordPayment: (id: string, payment: Omit<Payment, "id" | "at">, acceptSlip?: boolean) => void;
  rejectSlip: (id: string) => void;
  cancelBooking: (id: string, cancel: { reason: string; refund: number }) => void;

  addBlock: (b: Omit<Block, "id">) => void;
  removeBlock: (id: string) => void;
  setOverride: (propertyId: string, unitIds: string[], nights: ISODate[], o: RateOverride) => void;

  /** Marks one item, every item for a booking, or everything as read. */
  markRead: (id?: string, bookingId?: string) => void;
  log: (a: Omit<Activity, "id" | "at" | "read">) => void;

  invite: (m: Omit<TeamMember, "id" | "status" | "name"> & { name?: string }) => void;
  updateMember: (id: string, patch: Partial<TeamMember>) => void;
  removeMember: (id: string) => void;

  updateTemplate: (id: string, body: string) => void;
  setNotification: (event: NotificationEvent, channel: "push" | "whatsapp", on: boolean) => void;
  updateAccount: (patch: Partial<Account>) => void;
  reset: () => void;
};

const empty = {
  bookings: [] as Booking[],
  blocks: [] as Block[],
  overrides: {} as Record<string, Overrides>,
  activity: [] as Activity[],
  team: [] as TeamMember[],
  templates: DEFAULT_TEMPLATES,
  notifications: DEFAULT_NOTIFICATIONS,
  account: DEFAULT_ACCOUNT,
};

export const useData = create<DataState>()(
  persist(
    (set, get) => ({
      ...empty,

      startLocal: (p, account, withSample) => {
        const sample = withSample && p ? sampleData(p) : { bookings: [], blocks: [], activity: [] };
        const acc = { ...DEFAULT_ACCOUNT, ...account };
        set({
          ...empty,
          account: acc,
          bookings: sample.bookings,
          blocks: sample.blocks,
          activity: sample.activity,
          team:
            withSample && p
              ? sampleTeam(acc, p.id)
              : [
                  {
                    id: "tm_owner",
                    name: acc.name,
                    phone: acc.phone,
                    role: "owner",
                    propertyIds: [],
                    status: "active",
                  },
                ],
        });
      },

      addSample: (p, alt = false) => {
        const sample = sampleData(p, alt);
        set((s) => ({
          bookings: [...s.bookings, ...sample.bookings],
          blocks: [...s.blocks, ...sample.blocks],
          activity: [...sample.activity, ...s.activity].sort((a, b) => b.at.localeCompare(a.at)),
        }));
      },

      createBooking: (input, p) => {
        const booking: Booking = {
          ...input,
          id: uid("bk"),
          ref: p ? nextRef(p, get().bookings) : `SK-${Date.now().toString().slice(-4)}`,
          createdAt: new Date().toISOString(),
        };
        set((s) => ({ bookings: [...s.bookings, booking] }));
        return booking;
      },

      updateBooking: (id, patch) =>
        set((s) => ({ bookings: s.bookings.map((b) => (b.id === id ? { ...b, ...patch } : b)) })),

      setStatus: (id, status) => {
        const b = get().bookings.find((x) => x.id === id);
        if (!b) return;
        get().updateBooking(id, {
          status,
          requestExpiresAt: status === "requested" ? b.requestExpiresAt : undefined,
        });
        get().markRead(undefined, id);
        if (b.status === "requested" && status !== "requested") {
          get().log({
            kind: "request",
            title:
              status === "declined"
                ? `You declined ${b.guest.name}`
                : `You approved ${b.guest.name}`,
            subtitle:
              status === "declined"
                ? `${formatRange(b.checkIn, b.checkOut)}, dates are open again`
                : `${formatRange(b.checkIn, b.checkOut)}, payment link sent`,
            bookingId: id,
            propertyId: b.propertyId,
          });
        }
      },

      recordPayment: (id, payment, acceptSlip) => {
        const b = get().bookings.find((x) => x.id === id);
        if (!b) return;
        const payments = [
          ...b.payments,
          { ...payment, id: uid("pay"), at: new Date().toISOString() },
        ];
        const paid = payments.reduce(
          (n, x) => n + (x.method === "refund" ? -x.amount : x.amount),
          0,
        );
        get().updateBooking(id, {
          payments,
          slip: b.slip && acceptSlip ? { ...b.slip, status: "accepted" } : b.slip,
          status: b.status === "awaiting_payment" && paid > 0 ? "confirmed" : b.status,
        });
        get().log({
          kind: "payment",
          title: `Payment from ${b.guest.name}`,
          subtitle: `${b.ref}, recorded by you`,
          bookingId: id,
          propertyId: b.propertyId,
        });
        get().markRead(undefined, id);
      },

      rejectSlip: (id) => {
        const b = get().bookings.find((x) => x.id === id);
        if (b?.slip) get().updateBooking(id, { slip: { ...b.slip, status: "rejected" } });
      },

      cancelBooking: (id, cancel) => {
        const b = get().bookings.find((x) => x.id === id);
        if (!b) return;
        const payments =
          cancel.refund > 0
            ? [
                ...b.payments,
                {
                  id: uid("pay"),
                  amount: cancel.refund,
                  method: "refund" as const,
                  at: new Date().toISOString(),
                },
              ]
            : b.payments;
        get().updateBooking(id, {
          status: "cancelled",
          payments,
          cancel: { ...cancel, at: new Date().toISOString() },
        });
        get().log({
          kind: "cancellation",
          title: `${b.guest.name}'s booking cancelled`,
          subtitle: `${formatRange(b.checkIn, b.checkOut)}, dates are open again`,
          bookingId: id,
          propertyId: b.propertyId,
        });
      },

      addBlock: (b) => set((s) => ({ blocks: [...s.blocks, { ...b, id: uid("blk") }] })),
      removeBlock: (id) => set((s) => ({ blocks: s.blocks.filter((b) => b.id !== id) })),

      setOverride: (propertyId, unitIds, nights, o) =>
        set((s) => {
          const prop = { ...(s.overrides[propertyId] ?? {}) };
          for (const unitId of unitIds) {
            const unit = { ...(prop[unitId] ?? {}) };
            for (const night of nights) unit[night] = { ...unit[night], ...o };
            prop[unitId] = unit;
          }
          return { overrides: { ...s.overrides, [propertyId]: prop } };
        }),

      markRead: (id, bookingId) =>
        set((s) => ({
          activity: s.activity.map((a) =>
            id === undefined && bookingId === undefined
              ? { ...a, read: true }
              : a.id === id || (bookingId && a.bookingId === bookingId)
                ? { ...a, read: true }
                : a,
          ),
        })),

      log: (a) =>
        set((s) => ({
          activity: [
            { ...a, id: uid("act"), at: new Date().toISOString(), read: true },
            ...s.activity,
          ],
        })),

      invite: (m) =>
        set((s) => ({
          team: [...s.team, { ...m, name: m.name ?? "", id: uid("tm"), status: "invited" }],
        })),
      updateMember: (id, patch) =>
        set((s) => ({ team: s.team.map((m) => (m.id === id ? { ...m, ...patch } : m)) })),
      removeMember: (id) => set((s) => ({ team: s.team.filter((m) => m.id !== id) })),

      updateTemplate: (id, body) =>
        set((s) => ({ templates: s.templates.map((t) => (t.id === id ? { ...t, body } : t)) })),
      setNotification: (event, channel, on) =>
        set((s) => ({
          notifications: {
            ...s.notifications,
            [event]: { ...s.notifications[event], [channel]: on },
          },
        })),
      updateAccount: (patch) => set((s) => ({ account: { ...s.account, ...patch } })),
      reset: () => set({ ...empty }),
    }),
    {
      name: "staykey.data",
      storage: persistentStorage,
      // Version 2 moved properties to the API.
      version: 2,
      migrate: (persisted) => {
        const { properties: _properties, ...rest } = (persisted ?? {}) as Record<string, unknown>;
        return rest as unknown as DataState;
      },
    },
  ),
);

/** Today's date, read once per render so screens agree on what "today" is. */
export const useToday = () => today();
