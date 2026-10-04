import { create } from "zustand";
import { persist } from "zustand/middleware";
import { persistentStorage } from "@/lib/storage";

export type BookingType = "entire" | "rooms";
export type CurrencyCode = "USD" | "LKR";
export type Policy = "flexible" | "moderate" | "strict";
export type Channel = "airbnb" | "booking" | "agoda" | "expedia" | "whatsapp" | "none";

export type RoomType = {
  id: string;
  name: string;
  sleeps: number;
  count: number;
  /** Nightly rate as typed, in major units. */
  rate: string;
};

export type Photo = { uri: string; width?: number; height?: number };

export type Country = { code: string; dial: string; label: string };

export const COUNTRIES: Country[] = [
  { code: "LK", dial: "+94", label: "Sri Lanka" },
  { code: "GB", dial: "+44", label: "United Kingdom" },
  { code: "DE", dial: "+49", label: "Germany" },
  { code: "AU", dial: "+61", label: "Australia" },
  { code: "IN", dial: "+91", label: "India" },
  { code: "US", dial: "+1", label: "United States" },
];

export type Draft = {
  // You
  country: Country;
  phone: string;
  whatsappAlerts: boolean;
  firstName: string;
  lastName: string;

  // Your property
  propertyName: string;
  bookingType: BookingType;
  location: string;
  guests: number;
  bedrooms: number;
  beds: number;
  bathrooms: number;
  rooms: RoomType[];
  photos: Photo[];
  photosLater: boolean;

  // Getting paid
  currency: CurrencyCode;
  nightlyRate: string;
  weekendOn: boolean;
  weekendRate: string;
  checkIn: string;
  checkOut: string;
  bankOn: boolean;
  bankName: string;
  accountName: string;
  accountNumber: string;
  payAtProperty: boolean;
  depositPercent: number;
  policy: Policy;
  noSmoking: boolean;
  noParties: boolean;
  petsAllowed: boolean;

  // Go live
  channels: Channel[];
  slug: string;
  slugEdited: boolean;
};

export const initialDraft: Draft = {
  country: COUNTRIES[0] as Country,
  phone: "",
  whatsappAlerts: true,
  firstName: "",
  lastName: "",

  propertyName: "",
  bookingType: "entire",
  location: "",
  guests: 4,
  bedrooms: 2,
  beds: 2,
  bathrooms: 2,
  rooms: [],
  photos: [],
  photosLater: false,

  currency: "USD",
  nightlyRate: "",
  weekendOn: false,
  weekendRate: "",
  checkIn: "14:00",
  checkOut: "11:00",
  bankOn: true,
  bankName: "",
  accountName: "",
  accountNumber: "",
  payAtProperty: true,
  depositPercent: 30,
  policy: "moderate",
  noSmoking: true,
  noParties: true,
  petsAllowed: false,

  channels: [],
  slug: "",
  slugEdited: false,
};

type OnboardingState = {
  draft: Draft;
  /** Index into the current step list, so "Finish later" resumes in place. */
  stepId: string | null;
  update: (patch: Partial<Draft>) => void;
  setStep: (id: string | null) => void;
  clear: () => void;
};

export const useOnboarding = create<OnboardingState>()(
  persist(
    (set) => ({
      draft: initialDraft,
      stepId: null,
      update: (patch) => set((s) => ({ draft: { ...s.draft, ...patch } })),
      setStep: (stepId) => set({ stepId }),
      clear: () => set({ draft: initialDraft, stepId: null }),
    }),
    {
      name: "staykey.onboarding",
      storage: persistentStorage,
      version: 1,
      merge: (persisted, current) => {
        const p = persisted as Partial<OnboardingState> | undefined;
        return { ...current, ...p, draft: { ...initialDraft, ...(p?.draft ?? {}) } };
      },
    },
  ),
);

/** Lowest nightly rate across the draft, in minor units. */
export function baseRateMinor(d: Draft): number {
  if (d.bookingType === "rooms") {
    const rates = d.rooms.map((r) => Number(r.rate)).filter((n) => n > 0);
    return rates.length ? Math.round(Math.min(...rates) * 100) : 0;
  }
  const rate = Number(d.nightlyRate);
  return rate > 0 ? Math.round(rate * 100) : 0;
}

export const OTA_CHANNELS: Channel[] = ["airbnb", "booking", "agoda", "expedia"];

export function listsOnOTAs(d: Draft): boolean {
  return d.channels.some((c) => OTA_CHANNELS.includes(c));
}

export function formatTime(hhmm: string): string {
  const [h = 0, m = 0] = hhmm.split(":").map(Number);
  const suffix = h >= 12 ? "PM" : "AM";
  const hour = h % 12 === 0 ? 12 : h % 12;
  return `${hour}:${String(m).padStart(2, "0")} ${suffix}`;
}
