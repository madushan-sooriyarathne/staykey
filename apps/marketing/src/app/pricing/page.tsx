import type { Metadata } from "next";
import Link from "next/link";
import { Prices } from "@/components/prices";
import { type Question, Questions } from "@/components/questions";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { cachedPlans } from "@/lib/api";
import { pageMetadata } from "@/lib/metadata";
import { parsePeriod } from "@/lib/plans";

export const metadata: Metadata = pageMetadata({
  title: "Pricing for villas and guesthouses",
  description:
    "One flat plan for your villa or guesthouse, with no commission on bookings. Free until your first direct booking, for up to 60 days.",
  path: "/pricing",
});

const questions: Question[] = [
  {
    q: "Which plan do I need?",
    a: "Count the rooms or villas guests can book separately. A whole villa booked as one is a single unit. If you sell a whole house and its rooms separately, count the rooms.",
  },
  {
    q: "When do I start paying?",
    a: "StayKey is free until your first direct booking comes in, for up to 60 days. After that you have 7 days to pick a plan, and your booking page keeps taking bookings meanwhile.",
  },
  {
    q: "How do I pay?",
    a: (
      <>
        In the app, through the App Store or Google Play, which show the price in your own currency
        and renew until you cancel. Or on the web in rupees through Payments.lk, paid upfront for a
        month or a year, at{" "}
        <Link href="/billing" className="text-obsidian underline underline-offset-4">
          staykey.direct/billing
        </Link>
        .
      </>
    ),
  },
  {
    q: "Can I change plans?",
    a: "Yes. Pick another plan in the app under More, Subscription. The app suggests the plan that fits the rooms you have.",
  },
  {
    q: "What happens if I stop paying?",
    a: "Your booking page stops taking bookings and asks guests to message you on WhatsApp instead. The app, your calendar and your bookings keep working, and paying again reopens the page.",
  },
  {
    q: "Do guests pay anything to StayKey?",
    a: "No. Guests pay you directly by bank transfer, at the property, or by card through your own Payments.lk account.",
  },
];

export default async function PricingPage({ searchParams }: PageProps<"/pricing">) {
  const period = parsePeriod((await searchParams).period);
  const plans = await cachedPlans();
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-5 pt-10 pb-20 md:px-6 md:pt-16">
        <div className="flex max-w-3xl flex-col gap-4">
          <h1 className="font-semibold text-5xl text-obsidian leading-[1.08] md:text-heading-lg">
            One flat price, no commission
          </h1>
          <p className="text-body-lg text-steel">
            Plans only differ by how many rooms guests can book. Every direct booking is yours in
            full, and StayKey is free until your first one comes in.
          </p>
        </div>
        <div className="mt-12">
          <h2 className="sr-only">Plans</h2>
          <Prices
            plans={plans}
            period={period}
            switchHref={(p) => (p === "yearly" ? "/pricing?period=yearly" : "/pricing")}
          />
          <p className="mt-4 text-steel text-sm">
            Web prices in Sri Lankan rupees. In the app, the App Store and Google Play show their
            own price in your currency.
          </p>
        </div>
        <section aria-labelledby="questions" className="mx-auto mt-20 max-w-3xl">
          <h2 id="questions" className="mb-8 font-semibold text-3xl text-obsidian md:text-heading">
            Questions about plans
          </h2>
          <Questions items={questions} />
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
