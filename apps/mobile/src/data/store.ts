import { create } from "zustand";
import { persist } from "zustand/middleware";
import { persistentStorage } from "@/lib/storage";
import { today } from "./dates";
import { DEFAULT_ACCOUNT, DEFAULT_NOTIFICATIONS, DEFAULT_TEMPLATES, uid } from "./defaults";
import { sampleData, sampleTeam } from "./seed";
import type {
  Account,
  Activity,
  NotificationEvent,
  PropertyConfig,
  TeamMember,
  Template,
} from "./types";

/**
 * Data the API doesn't hold yet: activity, team, templates and notification settings. Properties
 * come from src/api/properties, and stays, blocks and rates from src/api/bookings and
 * src/api/calendar.
 */
type DataState = {
  activity: Activity[];
  team: TeamMember[];
  templates: Template[];
  notifications: Record<NotificationEvent, { push: boolean; whatsapp: boolean }>;
  account: Account;

  /** After publishing or signing in: the owner's details, plus a sample team and alerts when asked. */
  startLocal: (
    p: PropertyConfig | undefined,
    account: Partial<Account>,
    withSample: boolean,
  ) => void;
  /** Prototype only: sample alerts for another property, such as the sample guesthouse. */
  addSample: (p: PropertyConfig, alt?: boolean) => void;

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
  activity: [] as Activity[],
  team: [] as TeamMember[],
  templates: DEFAULT_TEMPLATES,
  notifications: DEFAULT_NOTIFICATIONS,
  account: DEFAULT_ACCOUNT,
};

export const useData = create<DataState>()(
  persist(
    (set) => ({
      ...empty,

      startLocal: (p, account, withSample) => {
        const acc = { ...DEFAULT_ACCOUNT, ...account };
        set({
          ...empty,
          account: acc,
          activity: withSample && p ? sampleAlerts(p) : [],
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

      addSample: (p, alt = false) =>
        set((s) => ({
          activity: [...sampleAlerts(p, alt), ...s.activity].sort((a, b) =>
            b.at.localeCompare(a.at),
          ),
        })),

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
      // Version 2 moved properties to the API, version 3 stays, blocks and rates.
      version: 3,
      migrate: (persisted) => {
        const {
          properties: _properties,
          bookings: _bookings,
          blocks: _blocks,
          overrides: _overrides,
          ...rest
        } = (persisted ?? {}) as Record<string, unknown>;
        return rest as unknown as DataState;
      },
    },
  ),
);

/**
 * The sample alerts. Their stays now live on the server (cmd/seed), so they don't link to one.
 */
function sampleAlerts(p: PropertyConfig, alt = false): Activity[] {
  return sampleData(p, alt).activity.map(({ bookingId: _bookingId, ...a }) => a);
}

/** Today's date, read once per render so screens agree on what "today" is. */
export const useToday = () => today();
