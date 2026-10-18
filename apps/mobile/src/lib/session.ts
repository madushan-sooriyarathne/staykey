import { create } from "zustand";
import { persist } from "zustand/middleware";
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
};

export const useSession = create<SessionState>()(
  persist(
    (set) => ({
      ...initial,
      hydrated: false,
      publish: (p) => set(p),
      completeOnboarding: () => set({ onboarded: true }),
      setUser: (userId) => set({ userId }),
      setAccount: (accountId, role) => set({ accountId, role }),
      signedOut: () => set({ onboarded: false }),
      setRole: (role) => set({ role }),
      reset: () => set({ ...initial }),
    }),
    {
      name: "staykey.session",
      storage: persistentStorage,
      // Version 2 dropped the plan kept on the phone and version 3 the photo; the server holds
      // both now.
      version: 3,
      migrate: (persisted) => {
        const {
          subscription: _s,
          startedAt: _t,
          photoUri: _p,
          ...rest
        } = (persisted ?? {}) as Record<string, unknown>;
        return rest;
      },
      partialize: ({ hydrated: _hydrated, ...rest }) => rest,
      merge: (persisted, current) => ({ ...current, ...(persisted as Partial<SessionState>) }),
      onRehydrateStorage: () => () => {
        useSession.setState({ hydrated: true });
      },
    },
  ),
);
