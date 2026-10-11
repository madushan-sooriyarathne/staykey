import type { Transition } from "@staykey/api-client";
import { useState } from "react";
import { useTransition } from "@/api/bookings";
import { messageFor } from "@/api/errors";
import type { Booking } from "@/data/types";
import { haptics } from "@/lib/haptics";

/** The statuses a person can move a stay to. Cancelling has its own screen. */
export type Next = Transition["to"];

/**
 * Moves a stay to its next status: approve, decline, check in or out. The change shows at once;
 * if the server refuses it the stay goes back and error explains why. The server logs answered
 * requests in Activity.
 */
export function useMoveStay() {
  const transition = useTransition();
  const [error, setError] = useState<string | null>(null);

  function move(b: Booking, to: Next) {
    setError(null);
    transition.mutate(
      { id: b.id, to, version: b.version },
      {
        onError: (e) => {
          haptics.error();
          setError(messageFor(e));
        },
      },
    );
  }

  return { move, error, clearError: () => setError(null) };
}
