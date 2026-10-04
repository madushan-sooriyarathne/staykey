import type { Account, SignIn } from "@staykey/api-client";
import { fetchProperties, propertyKeys } from "@/api/properties";
import { queryClient } from "@/api/query-client";
import { localProperty } from "@/data/from-api";
import { withSampleSettings } from "@/data/seed";
import { useData } from "@/data/store";
import { useOnboarding } from "@/features/onboarding/store";
import { PROTOTYPE } from "@/lib/prototype";
import { useSession } from "@/lib/session";
import { clearTokens } from "@/lib/tokens";

/** Owners land in their own account; team members in the first account they joined. */
export function chooseAccount(accounts: Account[]): Account | undefined {
  return accounts.find((a) => a.role === "owner") ?? accounts[0];
}

export type AfterSignIn =
  | { to: "app" }
  /** No account with a property yet: carry on with property setup. */
  | { to: "setup" }
  | { to: "error"; message: string };

/**
 * Decides where a verified sign-in goes. Someone with an account (a returning owner, or a team
 * member) goes straight to the app with that account's properties; a new owner carries on with
 * setup. Until phase 2 moves screens onto the API, properties are copied into the on-device
 * store, which is kept as it is when the same person signs back in.
 */
export async function afterSignIn(signIn: SignIn): Promise<AfterSignIn> {
  const session = useSession.getState();
  const sameUser = session.userId === signIn.user.id;
  if (!sameUser) {
    // Someone else's data and draft never carry over.
    useData.getState().reset();
    useOnboarding.getState().clear();
    queryClient.clear();
  }
  session.setUser(signIn.user.id);

  const account = chooseAccount(signIn.accounts);
  if (!account) {
    session.setAccount("", "owner");
    return { to: "setup" };
  }
  session.setAccount(account.id, account.role);

  const local = useData.getState().properties;
  if (sameUser && local.length > 0) return enterApp();

  let properties: Awaited<ReturnType<typeof fetchProperties>>;
  try {
    properties = await queryClient.fetchQuery({
      queryKey: propertyKeys.list(account.id),
      queryFn: fetchProperties,
      retry: 2,
    });
  } catch {
    // Without the properties there is nothing to show, so start the sign-in over.
    await clearTokens();
    return {
      to: "error",
      message: "Can't reach StayKey right now. Check your connection and send a new code.",
    };
  }

  // An owner whose account has no property yet stopped part way through setup.
  if (properties.length === 0 && account.role === "owner") return { to: "setup" };

  const [first, ...rest] = properties.map((p) => localProperty(p));
  const data = useData.getState();
  if (first) {
    data.start(first, { name: signIn.user.name, phone: signIn.user.phone }, PROTOTYPE);
    for (const p of rest) data.addProperty(PROTOTYPE ? withSampleSettings(p) : p);
  }
  useSession.setState({
    ownerName: signIn.user.name.split(" ")[0] ?? "",
    propertyName: first?.name ?? account.name,
    slug: first?.slug ?? "",
    bookingPageUrl: first?.bookingPageUrl ?? "",
  });
  return enterApp();
}

/** Flips the router guard to the tabs and drops the setup draft. */
function enterApp(): AfterSignIn {
  useSession.getState().completeOnboarding();
  useOnboarding.getState().clear();
  return { to: "app" };
}
