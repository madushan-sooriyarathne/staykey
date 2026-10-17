import type { Account, SignIn } from "@staykey/api-client";
import { create } from "zustand";
import { ApiError } from "@/api/errors";
import { fetchProperties, propertyKeys } from "@/api/properties";
import { queryClient } from "@/api/query-client";
import { acceptInvite } from "@/api/team";
import { toPropertyConfig } from "@/data/from-api";
import { useOnboarding } from "@/features/onboarding/store";
import { signOut } from "@/lib/auth";
import { identify } from "@/lib/purchases";
import { useSession } from "@/lib/session";

/** Owners land in their own account; team members in the first account they joined. */
export function chooseAccount(accounts: Account[]): Account | undefined {
  return accounts.find((a) => a.role === "owner") ?? accounts[0];
}

/**
 * Set while someone who was invited confirms their number. With the invite's token, the
 * verified sign-in accepts the invite and opens that account; without one, a number with no
 * account is told there's no invite rather than sent into property setup.
 */
export const useJoining = create<{ joining: boolean; token?: string }>(() => ({ joining: false }));

export type AfterSignIn =
  | { to: "app" }
  /** No account with a property yet: carry on with property setup. */
  | { to: "setup" }
  | { to: "error"; message: string };

const OFFLINE = "Can't reach StayKey right now. Check your connection and send a new code.";

/**
 * Decides where a verified sign-in goes. Someone with an account (a returning owner, or a team
 * member) goes straight to the app with that account's properties loaded; a new owner carries on
 * with setup. Cached data is kept when the same person signs back in, and dropped when it's
 * someone else.
 */
export async function afterSignIn(signIn: SignIn): Promise<AfterSignIn> {
  const session = useSession.getState();
  if (session.userId !== signIn.user.id) {
    // Someone else's stays and cached data never carry over. The setup draft stays: it is the
    // one this person is filling in right now (signing out clears it).
    queryClient.clear();
  }
  session.setUser(signIn.user.id);

  const { joining, token } = useJoining.getState();
  let account = chooseAccount(signIn.accounts);
  if (joining && token) {
    try {
      account = await acceptInvite(token);
    } catch (e) {
      await signOut();
      const gone = e instanceof ApiError && e.status === 404;
      return {
        to: "error",
        message: gone
          ? "This invite has expired, was already used, or was sent to another number. Ask the owner to send it again."
          : OFFLINE,
      };
    }
  }
  if (!account) {
    if (joining) {
      await signOut();
      return {
        to: "error",
        message:
          "We couldn't find an invite for this number. Ask the owner to send the invite again.",
      };
    }
    session.setAccount("", "owner");
    return { to: "setup" };
  }

  const opened = await openAccount(account, signIn.user.name);
  if (!opened) {
    // Without the properties there is nothing to show, so start the sign-in over.
    await signOut();
    return { to: "error", message: OFFLINE };
  }
  // An owner whose account has no property yet stopped part way through setup.
  if (opened.properties === 0 && account.role === "owner" && !joining) return { to: "setup" };

  useSession.getState().completeOnboarding();
  useOnboarding.getState().clear();
  useJoining.setState({ joining: false, token: undefined });
  return { to: "app" };
}

/**
 * Acts on an account from now on: loads its properties and the names the screens show. Returns
 * null when the properties can't be loaded.
 */
export async function openAccount(
  account: Account,
  userName = useSession.getState().ownerName,
): Promise<{ properties: number } | null> {
  const session = useSession.getState();
  session.setAccount(account.id, account.role);
  void identify(account.id);
  let properties: Awaited<ReturnType<typeof fetchProperties>>;
  try {
    properties = await queryClient.fetchQuery({
      queryKey: propertyKeys.list(account.id),
      queryFn: fetchProperties,
      retry: 2,
    });
  } catch {
    return null;
  }
  const first = properties[0] ? toPropertyConfig(properties[0]) : undefined;
  useSession.setState({
    ownerName: userName.split(" ")[0] ?? "",
    propertyName: first?.name ?? account.name,
    slug: first?.slug ?? "",
    bookingPageUrl: first?.bookingPageUrl ?? "",
  });
  return { properties: properties.length };
}
