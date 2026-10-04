export type StepId =
  | "phone"
  | "verify"
  | "name"
  | "basics"
  | "space"
  | "rooms"
  | "photos"
  | "price"
  | "payment"
  | "policy"
  | "channels"
  | "review";

export type StageId = "you" | "property" | "paid" | "live";

export type StepProps = {
  /** Advance to the next step, with forward motion and haptics. */
  next: () => void;
  /** Jump to an earlier step, with backward motion. */
  goTo: (id: StepId) => void;
  /** Error from the step's continue action, for example a taken booking page address. */
  error: string | null;
  clearError: () => void;
};
