import type { Account, SignIn } from "@staykey/api-client";
import { create } from "zustand";
import { fetchProperties, propertyKeys } from "@/api/properties";
import { queryClient } from "@/api/query-client";
import { toPropertyConfig } from "@/data/from-api";
import { useData } from "@/data/store";
import { useOnboarding } from "@/features/onboarding/store";
import { signOut } from "@/lib/auth";
import { PROTOTYPE } from "@/lib/prototype";
import { useSession } from "@/lib/session";

/** Owners land in their own account; team members in the first account they joined. */
export function chooseAccount(accounts: Account[]): Account | undefined {
  return accounts.find((a) => a.role === "owner") ?? accounts[0];
}

/**
 * Set while someone who was invited confirms their number, so a number without an account is
 * told there's no invite rather than sent into property setup.
 */
export const useJoining = create<{ joining: boolean }>(() => ({ joining: false }));

export type AfterSignIn =
  | { to: "app" }
  /** No account with a property yet: carry on with property setup. */
  | { to: "setup" }
  | { to: "error"; message: string };

/**
 * Decides where a verified sign-in goes. Someone with an account (a returning owner, or a team
 * member) goes straight to the app with that account's properties loaded; a new owner carries on
 * with setup. Bookings and the rest of the on-device data are kept when the same person signs
 * back in, and dropped when it's someone else.
 */
export async function afterSignIn(signIn: SignIn): Promise<AfterSignIn> {
  const session = useSession.getState();
  const sameUser = session.userId === signIn.user.id;
  if (!sameUser) {
    // Someone else's stays and cached data never carry over. The setup draft stays: it is the
    // one this person is filling in right now (signing out clears it).
    useData.getState().reset();
    queryClient.clear();
  }
  session.setUser(signIn.user.id);

  const joining = useJoining.getState().joining;
  const account = chooseAccount(signIn.accounts);
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
  session.setAccount(account.id, account.role);

  let properties: Awaited<ReturnType<typeof fetchProperties>>;
  try {
    properties = await queryClient.fetchQuery({
      queryKey: propertyKeys.list(account.id),
      queryFn: fetchProperties,
      retry: 2,
    });
  } catch {
    // Without the properties there is nothing to show, so start the sign-in over.
    await signOut();
    return {
      to: "error",
      message: "Can't reach StayKey right now. Check your connection and send a new code.",
    };
  }

  // An owner whose account has no property yet stopped part way through setup.
  if (properties.length === 0 && account.role === "owner" && !joining) return { to: "setup" };

  const first = properties[0] ? toPropertyConfig(properties[0]) : undefined;
  if (!sameUser || useData.getState().bookings.length === 0) {
    useData
      .getState()
      .startLocal(first, { name: signIn.user.name, phone: signIn.user.phone }, PROTOTYPE);
  }
  useSession.setState({
    ownerName: signIn.user.name.split(" ")[0] ?? "",
    propertyName: first?.name ?? account.name,
    slug: first?.slug ?? "",
    bookingPageUrl: first?.bookingPageUrl ?? "",
  });
  useSession.getState().completeOnboarding();
  useOnboarding.getState().clear();
  useJoining.setState({ joining: false });
  return { to: "app" };
}
