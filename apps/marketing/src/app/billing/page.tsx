import type { Subscription } from "@staykey/api-client";
import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";
import { ownedAccounts, plans, subscription } from "@/lib/api";
import { formatDate, formatRupees } from "@/lib/format";
import { ACCESS_COOKIE } from "@/lib/session";
import { signOutOfBilling } from "./actions";
import { type PlanChoice, PlanForm } from "./plan-form";
import { Shell } from "./shell";
import { SignInForm } from "./sign-in-form";

export const metadata: Metadata = {
  title: "Your StayKey plan",
  robots: { index: false, follow: false },
};

/** Where the plan stands, in a sentence for the owner. */
function standing(sub: Subscription): string {
  const until = formatDate(sub.until);
  switch (sub.status) {
    case "trial":
      return `You're in your free period until ${until}. Your first direct booking ends it sooner.`;
    case "active":
      return sub.source === "web"
        ? `Paid until ${until}. Paying again adds a period after that date.`
        : "Your plan renews through the App Store or Google Play. Manage it there.";
    case "grace":
      return `Choose a plan by ${until} to keep your booking page open.`;
    case "lapsed":
      return "Your booking page is paused. Guests are asked to message you on WhatsApp until you choose a plan.";
    default:
      return "This account is closed.";
  }
}

function SignOut() {
  return (
    <form action={signOutOfBilling}>
      <button type="submit" className="text-steel underline">
        Sign out
      </button>
    </form>
  );
}

/**
 * The web billing page: sign in with the phone number, see the plan, and pay for a month or a
 * year through Payments.lk. The apps never link here, since the stores don't allow pointing to
 * outside payment.
 */
export default async function BillingPage(props: PageProps<"/billing">) {
  const token = (await cookies()).get(ACCESS_COOKIE)?.value;
  if (!token) {
    return (
      <Shell title="Your StayKey plan">
        <p className="text-steel">Sign in to choose or renew the plan for your booking page.</p>
        <SignInForm />
      </Shell>
    );
  }

  const accounts = await ownedAccounts(token);
  if (!accounts.ok) {
    return (
      <Shell title="Your StayKey plan">
        <p className="text-steel">{accounts.message}</p>
        <SignOut />
      </Shell>
    );
  }
  if (accounts.data.length === 0) {
    return (
      <Shell title="No account to pay for">
        <p className="text-steel">
          This number isn't the owner of a StayKey account. The account owner chooses the plan.
        </p>
        <SignOut />
      </Shell>
    );
  }

  const { account: asked } = await props.searchParams;
  const account = accounts.data.find((a) => a.id === asked) ?? accounts.data[0];
  if (!account) return null;
  const [sub, list] = await Promise.all([subscription(token, account.id), plans()]);
  if (!sub.ok) {
    return (
      <Shell title="Your StayKey plan">
        <p className="text-steel">{sub.message}</p>
        <SignOut />
      </Shell>
    );
  }

  const canPay = sub.data.canPurchase || sub.data.source === "web";
  const choices: PlanChoice[] = list.map((p) => ({
    id: p.id,
    name: p.name,
    units: p.units === 1 ? "1 villa or room" : `Up to ${p.units} rooms`,
    monthly: `${formatRupees(p.monthly, p.currency)} a month`,
    yearly: `${formatRupees(p.yearly, p.currency)} a year`,
  }));
  const fitsName = list.find((p) => p.id === sub.data.fits)?.name ?? "";
  const tooSmall = sub.data.plan && sub.data.plan !== sub.data.fits;

  return (
    <Shell title={account.name}>
      {accounts.data.length > 1 && (
        <nav className="flex flex-wrap gap-2">
          {accounts.data.map((a) => (
            <Link
              key={a.id}
              href={`/billing?account=${a.id}`}
              className={`rounded-button border px-3 py-2 text-caption ${a.id === account.id ? "border-obsidian text-obsidian" : "border-mist text-steel"}`}
            >
              {a.name}
            </Link>
          ))}
        </nav>
      )}
      <p className="text-steel">{standing(sub.data)}</p>
      {tooSmall && (
        <p className="text-steel">
          Your properties have {sub.data.units} bookable units, which the {fitsName} plan covers.
        </p>
      )}
      {canPay && (
        <PlanForm
          account={account.id}
          plans={choices}
          fits={sub.data.fits}
          submitLabel={
            sub.data.status === "active" ? "Pay for the next period" : "Continue to payment"
          }
        />
      )}
      <SignOut />
    </Shell>
  );
}
