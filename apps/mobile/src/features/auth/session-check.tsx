import { useEffect } from "react";
import { ApiError } from "@/api/errors";
import { useMe } from "@/api/me";
import { useSession } from "@/lib/session";

/**
 * Confirms the session with the API when the app opens signed in: an ended session (the refresh
 * token was refused) or an account the user no longer belongs to sends them back to Welcome.
 * Builds from before real sign-in have no user, so they are left alone.
 */
export function SessionCheck() {
  const onboarded = useSession((s) => s.onboarded);
  const userId = useSession((s) => s.userId);
  const accountId = useSession((s) => s.accountId);
  const me = useMe({ enabled: onboarded && Boolean(userId) });

  useEffect(() => {
    const signedOut = useSession.getState().signedOut;
    if (me.error instanceof ApiError && me.error.status === 401) {
      signedOut();
      return;
    }
    if (me.data && accountId && !me.data.accounts.some((a) => a.id === accountId)) signedOut();
  }, [me.data, me.error, accountId]);

  return null;
}
