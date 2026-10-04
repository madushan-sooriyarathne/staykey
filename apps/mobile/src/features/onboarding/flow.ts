import { SLUG_PATTERN } from "@staykey/api-client";
import type { ComponentType } from "react";
import { api } from "@/lib/api";
import { ChannelsStep, ReviewStep } from "./steps/live";
import { PaymentStep, PolicyStep, PriceStep } from "./steps/paid";
import { BasicsStep, PhotosStep, RoomsStep, SpaceStep } from "./steps/property";
import { isValidPhone, NameStep, PhoneStep, VerifyStep } from "./steps/you";
import { baseRateMinor, type Draft } from "./store";
import type { StageId, StepId, StepProps } from "./types";

export const STAGES: { id: StageId; label: string }[] = [
  { id: "you", label: "You" },
  { id: "property", label: "Your property" },
  { id: "paid", label: "Getting paid" },
  { id: "live", label: "Go live" },
];

export type StepDef = {
  id: StepId;
  stage: StageId;
  title: (d: Draft) => string;
  subtitle?: (d: Draft) => string;
  /** Footer button label. Steps without one (the code step) advance on their own. */
  cta?: (d: Draft) => string;
  canContinue: (d: Draft) => boolean;
  /** Optional secondary action under the button. */
  secondary?: { label: string; patch: Partial<Draft> };
  Component: ComponentType<StepProps>;
};

const ALL_STEPS: StepDef[] = [
  {
    id: "phone",
    stage: "you",
    title: () => "What's your phone number?",
    subtitle: () => "We'll text you a 6-digit code to sign you in.",
    cta: () => "Send code",
    canContinue: (d) => isValidPhone(d.country, d.phone),
    Component: PhoneStep,
  },
  {
    id: "verify",
    stage: "you",
    title: () => "Enter the 6-digit code",
    canContinue: () => false,
    Component: VerifyStep,
  },
  {
    id: "name",
    stage: "you",
    title: () => "What should guests call you?",
    subtitle: () => "Your name appears on booking emails as the host.",
    cta: () => "Continue",
    canContinue: (d) => d.firstName.trim().length > 0,
    Component: NameStep,
  },
  {
    id: "basics",
    stage: "property",
    title: () => "Tell us about your property",
    cta: () => "Continue",
    canContinue: (d) => d.propertyName.trim().length >= 2,
    Component: BasicsStep,
  },
  {
    id: "space",
    stage: "property",
    title: (d) => `How big is ${d.propertyName.trim() || "your place"}?`,
    subtitle: () => "This sets how many guests can book at once.",
    cta: () => "Continue",
    canContinue: () => true,
    Component: SpaceStep,
  },
  {
    id: "rooms",
    stage: "property",
    title: () => "Add your rooms",
    subtitle: () => "Group identical rooms into one type, like 2 Garden Rooms.",
    cta: () => "Continue",
    canContinue: (d) => d.rooms.length > 0,
    Component: RoomsStep,
  },
  {
    id: "photos",
    stage: "property",
    title: () => "Add photos guests will love",
    subtitle: () => "Add at least 3. The first one is your cover.",
    cta: (d) =>
      d.photos.length >= 3
        ? `Continue with ${d.photos.length} photos`
        : `Add ${3 - d.photos.length} more ${3 - d.photos.length === 1 ? "photo" : "photos"}`,
    canContinue: (d) => d.photos.length >= 3 || d.photosLater,
    secondary: { label: "I'll add photos later", patch: { photosLater: true } },
    Component: PhotosStep,
  },
  {
    id: "price",
    stage: "paid",
    title: () => "Set your nightly price",
    subtitle: () => "Seasons and discounts can come later.",
    cta: () => "Continue",
    canContinue: (d) => Number(d.nightlyRate) > 0 && (!d.weekendOn || Number(d.weekendRate) > 0),
    Component: PriceStep,
  },
  {
    id: "payment",
    stage: "paid",
    title: () => "How should guests pay you?",
    subtitle: () => "Both work today, with no approval needed.",
    cta: () => "Continue",
    canContinue: (d) =>
      (d.bankOn || d.payAtProperty) &&
      (!d.bankOn ||
        (d.bankName !== "" && d.accountName.trim().length > 1 && d.accountNumber.length >= 6)),
    Component: PaymentStep,
  },
  {
    id: "policy",
    stage: "paid",
    title: () => "Pick a cancellation policy",
    subtitle: () => "Guests agree to it before they book.",
    cta: () => "Continue",
    canContinue: () => true,
    Component: PolicyStep,
  },
  {
    id: "channels",
    stage: "live",
    title: () => "Where else do guests book you?",
    subtitle: () => "Pick every place you take bookings today.",
    cta: () => "Continue",
    canContinue: (d) => d.channels.length > 0,
    Component: ChannelsStep,
  },
  {
    id: "review",
    stage: "live",
    title: () => "Ready to go live",
    cta: () => "Publish my booking page",
    canContinue: (d) => SLUG_PATTERN.test(d.slug),
    Component: ReviewStep,
  },
];

/** The steps for this owner. Room-based properties set prices per room, so they skip "price". */
export function stepsFor(d: Draft): StepDef[] {
  return ALL_STEPS.filter((s) => {
    if (s.id === "space") return d.bookingType === "entire";
    if (s.id === "rooms" || s.id === "price")
      return d.bookingType === (s.id === "rooms" ? "rooms" : "entire");
    return true;
  });
}

/**
 * Overall progress across the four stages, from 0 to 4. A stage's bar fills in proportion to
 * the steps completed inside it and is full once the next stage starts.
 */
export function progressFor(steps: StepDef[], index: number): number {
  const step = steps[index];
  if (!step) return 0;
  const stageIndex = STAGES.findIndex((s) => s.id === step.stage);
  const inStage = steps.filter((s) => s.stage === step.stage);
  const position = inStage.findIndex((s) => s.id === step.id);
  return stageIndex + (position + 1) / (inStage.length + 1);
}

export type PublishResult =
  | { ok: true; slug: string; bookingPageUrl: string }
  | { ok: false; field?: "slug"; message: string };

/** Creates the property, which also reserves its booking page address. */
export async function publish(d: Draft): Promise<PublishResult> {
  try {
    const { data, error, response } = await api.POST("/v1/properties", {
      body: {
        name: d.propertyName.trim(),
        slug: d.slug,
        bookingType: d.bookingType,
        location: d.location.trim() || undefined,
        currency: d.currency,
        baseRate: baseRateMinor(d),
      },
    });
    if (data) return { ok: true, slug: data.slug, bookingPageUrl: data.bookingPageUrl };
    if (response.status === 409) {
      return { ok: false, field: "slug", message: "That address is taken. Try another." };
    }
    if (error?.code === "invalid_slug") return { ok: false, field: "slug", message: error.message };
    return { ok: false, message: error?.message ?? "Publishing didn't work. Please try again." };
  } catch {
    return {
      ok: false,
      message: "Can't reach StayKey right now. Check your connection and try again.",
    };
  }
}
