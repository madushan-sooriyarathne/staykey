import { create } from "zustand";
import { persist } from "zustand/middleware";
import type { Period, PlanId } from "./purchases";
import { persistentStorage } from "./storage";

export type Role = "owner" | "manager" | "caretaker";

/** What onboarding learned that later screens need: the checklist and the paywall. */
export type Setup = {
  /** Bookable units: 1 for a whole place, the sum of room counts otherwise. */
  units: number;
  currency: "USD" | "LKR";
  /** Lowest nightly rate in minor units. */
  nightlyRate: number;
  photoCount: number;
  /** OTA names the owner listed, so the checklist can lead with calendar sync. */
  otas: string[];
};

export type Subscription = { status: "trial" } | { status: "active"; plan: PlanId; period: Period };

type SessionState = {
  /** True once the owner has finished onboarding (or joined a team) and lands on the tabs. */
  onboarded: boolean;
  /**
   * The last user who signed in on this device, set by a verified code. It survives an expired
   * session so signing back in as the same person keeps their local data.
   */
  userId: string;
  /** The account owner routes act on, sent as X-Account-Id. */
  accountId: string;
  role: Role;
  ownerName: string;
  propertyName: string;
  slug: string;
  bookingPageUrl: string;
  setup: Setup;
  subscription: Subscription;
  /** When the owner published, which starts the free period. */
  startedAt: string;
  /** Set when the persisted state has loaded, so the router doesn't flash the wrong stack. */
  hydrated: boolean;

  publish: (p: {
    ownerName: string;
    propertyName: string;
    slug: string;
    bookingPageUrl: string;
    setup: Setup;
  }) => void;
  completeOnboarding: () => void;
  /** After a verified code: who signed in. */
  setUser: (userId: string) => void;
  /** Acts on this account from now on, with the caller's role in it. */
  setAccount: (accountId: string, role: Role) => void;
  /** The server ended the session (the refresh token was refused): back to Welcome to sign in. */
  signedOut: () => void;
  joinTeam: (p: { role: Role; propertyName: string }) => void;
  subscribe: (plan: PlanId, period: Period) => void;
  /** Prototype only: preview the app as another role. */
  setRole: (role: Role) => void;
  reset: () => void;
};

const initial = {
  onboarded: false,
  userId: "",
  accountId: "",
  role: "owner" as Role,
  ownerName: "",
  propertyName: "",
  slug: "",
  bookingPageUrl: "",
  setup: { units: 1, currency: "USD", nightlyRate: 0, photoCount: 0, otas: [] } as Setup,
  subscription: { status: "trial" } as Subscription,
  startedAt: "",
};

export const useSession = create<SessionState>()(
  persist(
    (set) => ({
      ...initial,
      hydrated: false,
      publish: (p) => set({ ...p, startedAt: new Date().toISOString() }),
      completeOnboarding: () => set({ onboarded: true }),
      setUser: (userId) => set({ userId }),
      setAccount: (accountId, role) => set({ accountId, role }),
      signedOut: () => set({ onboarded: false }),
      joinTeam: ({ role, propertyName }) => set({ onboarded: true, role, propertyName }),
      subscribe: (plan, period) => set({ subscription: { status: "active", plan, period } }),
      setRole: (role) => set({ role }),
      reset: () => set({ ...initial }),
    }),
    {
      name: "staykey.session",
      storage: persistentStorage,
      version: 1,
      partialize: ({ hydrated: _hydrated, ...rest }) => rest,
      merge: (persisted, current) => ({ ...current, ...(persisted as Partial<SessionState>) }),
      onRehydrateStorage: () => () => {
        useSession.setState({ hydrated: true });
      },
    },
  ),
);
