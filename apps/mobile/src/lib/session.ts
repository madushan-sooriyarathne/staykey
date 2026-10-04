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
  role: Role;
  ownerName: string;
  propertyName: string;
  slug: string;
  bookingPageUrl: string;
  setup: Setup;
  subscription: Subscription;
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
  joinTeam: (p: { role: Role; propertyName: string }) => void;
  subscribe: (plan: PlanId, period: Period) => void;
  reset: () => void;
};

const initial = {
  onboarded: false,
  role: "owner" as Role,
  ownerName: "",
  propertyName: "",
  slug: "",
  bookingPageUrl: "",
  setup: { units: 1, currency: "USD", nightlyRate: 0, photoCount: 0, otas: [] } as Setup,
  subscription: { status: "trial" } as Subscription,
};

export const useSession = create<SessionState>()(
  persist(
    (set) => ({
      ...initial,
      hydrated: false,
      publish: (p) => set({ ...p }),
      completeOnboarding: () => set({ onboarded: true }),
      joinTeam: ({ role, propertyName }) => set({ onboarded: true, role, propertyName }),
      subscribe: (plan, period) => set({ subscription: { status: "active", plan, period } }),
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
