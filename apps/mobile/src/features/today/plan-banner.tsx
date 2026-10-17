import type { Subscription } from "@staykey/api-client";
import { router } from "expo-router";
import { Pressable } from "react-native";
import { useSubscription } from "@/api/subscription";
import { InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { formatDay, toISO } from "@/data/dates";
import { useCan } from "@/data/hooks";
import { PLANS } from "@/lib/purchases";

/** What the owner should hear about the plan on Today, or nothing. */
function notice(sub: Subscription): string | null {
  switch (sub.status) {
    case "grace":
      return `Choose a plan by ${formatDay(toISO(new Date(sub.until)))} to keep your booking page open.`;
    case "lapsed":
      return "Your booking page is paused. Choose a plan to open it again.";
    case "active": {
      const paid = PLANS.find((p) => p.id === sub.plan);
      const fits = PLANS.find((p) => p.id === sub.fits);
      return paid && fits && fits.units > paid.units
        ? `Your properties have ${sub.units} bookable units. The ${fits.name} plan covers them.`
        : null;
    }
    default:
      return null;
  }
}

/** A nudge for owners whose plan needs them; it opens the paywall. Nothing is ever blocked. */
export function PlanBanner() {
  const can = useCan();
  const sub = useSubscription().data;
  const text = can("billing") && sub ? notice(sub) : null;
  if (!text) return null;
  return (
    <Pressable
      testID="today-plan"
      accessibilityRole="button"
      onPress={() => router.push("/paywall")}
    >
      <InfoNote icon={I.crown}>{text}</InfoNote>
    </Pressable>
  );
}
