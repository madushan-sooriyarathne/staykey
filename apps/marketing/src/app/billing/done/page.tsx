import type { Metadata } from "next";
import { cookies } from "next/headers";
import Link from "next/link";
import { subscription } from "@/lib/api";
import { formatDate } from "@/lib/format";
import { ACCESS_COOKIE, ACCOUNT_COOKIE } from "@/lib/session";
import { Shell } from "../shell";

export const metadata: Metadata = {
  title: "Your StayKey plan",
  robots: { index: false, follow: false },
};

/**
 * Where Payments.lk returns the owner. Its status in the address only says how the checkout
 * page ended; the plan counts once Payments.lk tells the API, so this page reads the plan back.
 */
export default async function BillingDonePage(props: PageProps<"/billing/done">) {
  const { status } = await props.searchParams;
  if (status !== "succeeded") {
    return (
      <Shell title="The payment didn't go through">
        <p className="text-steel">
          Nothing was charged. You can try again with the same or another card.
        </p>
        <Link
          href="/billing"
          className="rounded-button bg-obsidian px-5 py-4 text-center text-snow shadow-button"
        >
          Back to your plan
        </Link>
      </Shell>
    );
  }

  const jar = await cookies();
  const token = jar.get(ACCESS_COOKIE)?.value;
  const account = jar.get(ACCOUNT_COOKIE)?.value;
  const sub = token && account ? await subscription(token, account) : undefined;
  if (sub?.ok && sub.data.status === "active" && sub.data.source === "web") {
    return (
      <Shell title="Thank you">
        <p className="text-steel">
          Your plan is paid until {formatDate(sub.data.until)}. Your booking page takes bookings as
          usual.
        </p>
        <Link href="/billing" className="text-steel underline">
          Back to your plan
        </Link>
      </Shell>
    );
  }

  return (
    <Shell title="Almost there">
      <p className="text-steel">
        We haven't heard from the bank yet. This usually takes a few seconds.
      </p>
      <Link
        href="/billing/done?status=succeeded"
        className="rounded-button bg-obsidian px-5 py-4 text-center text-snow shadow-button"
      >
        Check again
      </Link>
    </Shell>
  );
}
