import type { Transition } from "@staykey/api-client";
import { useState } from "react";
import { useTransition } from "@/api/bookings";
import { messageFor } from "@/api/errors";
import { formatRange } from "@/data/dates";
import { useData } from "@/data/store";
import type { Booking } from "@/data/types";
import { haptics } from "@/lib/haptics";

/** The statuses a person can move a stay to. Cancelling has its own screen. */
export type Next = Transition["to"];

/**
 * Moves a stay to its next status: approve, decline, check in or out. The change shows at once;
 * if the server refuses it the stay goes back and error explains why. Answering a request is
 * logged in Activity, which stays on the device until phase 4.
 */
export function useMoveStay() {
  const transition = useTransition();
  const log = useData((s) => s.log);
  const markRead = useData((s) => s.markRead);
  const [error, setError] = useState<string | null>(null);

  function move(b: Booking, to: Next) {
    setError(null);
    markRead(undefined, b.id);
    transition.mutate(
      { id: b.id, to, version: b.version },
      {
        onSuccess: () => {
          if (b.status !== "requested") return;
          const declined = to === "declined";
          log({
            kind: "request",
            title: `${declined ? "You declined" : "You approved"} ${b.guest.name}`,
            subtitle: `${formatRange(b.checkIn, b.checkOut)}, ${declined ? "dates are open again" : "payment link sent"}`,
            bookingId: b.id,
            propertyId: b.propertyId,
          });
        },
        onError: (e) => {
          haptics.error();
          setError(messageFor(e));
        },
      },
    );
  }

  return { move, error, clearError: () => setError(null) };
}
