import type { Metadata } from "next";
import Link from "next/link";
import { PhoneFrame } from "@/components/phone-frame";
import { Prices } from "@/components/prices";
import { type Question, Questions } from "@/components/questions";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { StoreButtons } from "@/components/store-buttons";
import { cachedPlans } from "@/lib/api";
import { SUPPORT } from "@/lib/company";
import { structuredData } from "@/lib/structured-data";

export const metadata: Metadata = {
  alternates: { canonical: "/" },
};

// Prerendered and refreshed every 10 minutes, so visitors get it straight from the cache. A
// build without the API shows "See prices in the app" until the first refresh.
export const revalidate = 600;

const steps = [
  {
    title: "Install and sign in",
    body: "Download StayKey and sign in with your phone number. No passwords, no laptop needed.",
  },
  {
    title: "Add your property",
    body: "Photos, rooms, rates, house rules and how guests pay. It takes about 10 minutes.",
  },
  {
    title: "Share your link",
    body: "Your booking page is live straight away. Send it on WhatsApp, put it on Instagram and Google, and connect your Airbnb calendar.",
  },
];

const features = [
  {
    title: "Your own booking page",
    body: "yourvilla.staykey.direct, with your photos, rooms, rates and rules. Guests pick dates on live availability and book, or send a request for you to approve.",
  },
  {
    title: "A widget for your website",
    body: "Paste one line into your site and guests check dates and book there, without leaving it.",
  },
  {
    title: "One calendar for every booking",
    body: "Direct stays, the WhatsApp bookings you add, and your Airbnb and Booking.com calendars side by side. StayKey reads them every 10 minutes and gives them a link to read yours, so the same night isn't sold twice.",
  },
  {
    title: "Get paid your way",
    body: "Bank transfer with the guest's slip uploaded to the booking, payment at the property, or cards through your own Payments.lk account. StayKey takes no commission on any of them.",
  },
  {
    title: "Your team, with the right access",
    body: "Managers run bookings with you. Caretakers see arrivals, guest notes and contact details, but never prices or payments.",
  },
  {
    title: "Guests hear from you",
    body: "Confirmations, payment reminders, pre-arrival notes and a thank-you after checkout, by email and WhatsApp, written from your own templates.",
  },
];

const problems = [
  "Bookings scattered across WhatsApp, Excel and OTA inboxes",
  "Commission on guests who would have booked direct",
  "Double bookings when calendars live in different places",
];

const questions: Question[] = [
  {
    q: "Does StayKey take a commission?",
    a: "No. You pay a flat plan for your property, and every direct booking is yours in full.",
  },
  {
    q: "How long is it free?",
    a: "StayKey is free until your first direct booking comes in, for up to 60 days. After that, pick the plan that fits your rooms.",
  },
  {
    q: "Will it work with Airbnb and Booking.com?",
    a: "Yes. Paste each listing's calendar link into StayKey, and paste your StayKey link back into them. Their stays show in your calendar and close those dates on your booking page.",
  },
  {
    q: "Can guests pay by card?",
    a: "Yes, through your own Payments.lk account, which needs a business registered in Sri Lanka. Guests pay in rupees on Payments.lk's secure page and the money goes straight to you. StayKey adds no charge.",
  },
  {
    q: "Who owns my data?",
    a: "You do. Download everything from the app at any time, and delete your account whenever you like.",
  },
  {
    q: "How do I cancel?",
    a: (
      <>
        A plan bought in the app is cancelled in the App Store or Google Play. A plan bought on the
        web is paid upfront for a month or a year and simply ends. See{" "}
        <Link href="/pricing" className="text-obsidian underline underline-offset-4">
          pricing
        </Link>{" "}
        for the details.
      </>
    ),
  },
];

export default async function Home() {
  const plans = await cachedPlans();
  return (
    <>
      <script
        type="application/ld+json"
        // Built from our own constants and the API's plans, then escaped by structuredData.
        // biome-ignore lint/security/noDangerouslySetInnerHtml: JSON-LD needs a raw script body
        dangerouslySetInnerHTML={{ __html: structuredData(plans) }}
      />
      <SiteHeader />

      <main>
        <section className="mx-auto grid max-w-6xl items-center gap-12 px-5 pt-10 pb-20 md:grid-cols-[1.25fr_1fr] md:px-6 md:pt-16">
          <div className="flex flex-col gap-7">
            <h1 className="font-semibold text-5xl text-obsidian leading-[1.08] md:text-display">
              Take direct bookings in 10 minutes
            </h1>
            <p className="max-w-xl text-body-lg text-steel">
              StayKey gives villa and guesthouse owners in Sri Lanka their own booking page, a
              website widget and one calendar for every booking, all set up from your phone.
            </p>
            <StoreButtons />
            <p className="text-steel text-sm">
              Free until your first direct booking. No commission, ever.
            </p>
          </div>
          <div className="relative flex justify-center md:justify-end">
            <PhoneFrame
              src="/screens/today.webp"
              alt="StayKey's Today screen: today's arrivals, departures and guests in house, and a booking request to approve or decline"
              eager
            />
            <div className="absolute bottom-10 left-0 hidden max-w-[240px] rounded-card border border-cloud bg-snow p-4 md:block lg:-left-6">
              <p className="text-caption text-steel">Your booking page</p>
              <p className="mt-1 font-medium text-obsidian">kingfisher.staykey.direct</p>
              <p className="mt-2 flex items-center gap-2 text-caption text-ember-ink">
                <span aria-hidden className="size-2 rounded-full bg-ember" />
                New direct booking, Emma Larsen
              </p>
            </div>
          </div>
        </section>

        <section aria-labelledby="how" className="mx-auto max-w-6xl px-5 pb-20 md:px-6">
          <h2 id="how" className="font-semibold text-3xl text-obsidian md:text-heading">
            How it works
          </h2>
          <ol className="mt-8 grid gap-8 md:grid-cols-3">
            {steps.map((s, i) => (
              <li key={s.title} className="flex flex-col gap-3 border-cloud border-t pt-6">
                <span className="font-semibold text-ember-ink">Step {i + 1}</span>
                <h3 className="font-semibold text-obsidian text-xl">{s.title}</h3>
                <p className="text-steel">{s.body}</p>
              </li>
            ))}
          </ol>
        </section>

        <section
          id="features"
          aria-labelledby="features-title"
          className="mx-auto grid max-w-6xl scroll-mt-6 gap-10 px-5 pb-20 md:grid-cols-[1fr_1.6fr] md:px-6"
        >
          <div className="flex flex-col gap-6 md:sticky md:top-8 md:self-start">
            <h2
              id="features-title"
              className="font-semibold text-3xl text-obsidian md:text-heading"
            >
              Everything a small property needs
            </h2>
            <p className="text-steel">
              Every plan includes all of it. Plans only differ by how many rooms guests can book.
            </p>
            <div className="hidden md:block">
              <PhoneFrame
                src="/screens/calendar.webp"
                alt="StayKey's calendar for a villa: direct bookings, nights booked on other sites, blocked nights and the price of each free night"
              />
            </div>
          </div>
          <ul className="divide-y divide-cloud border-cloud border-y">
            {features.map((f) => (
              <li key={f.title} className="grid gap-2 py-6 md:grid-cols-[14rem_1fr] md:gap-8">
                <h3 className="font-semibold text-obsidian">{f.title}</h3>
                <p className="text-steel">{f.body}</p>
              </li>
            ))}
          </ul>
        </section>

        <section className="mx-auto max-w-6xl px-5 pb-20 md:px-6">
          <div className="relative overflow-hidden rounded-card-lg bg-graphite p-8 text-snow md:p-12">
            <div
              aria-hidden
              className="pointer-events-none absolute -top-24 -right-20 size-72 rounded-full bg-[radial-gradient(circle,rgba(254,69,226,0.5),rgba(255,90,0,0.3)_40%,transparent_70%)]"
            />
            <h2 className="relative font-bold text-3xl md:text-4xl">What StayKey replaces</h2>
            <ul className="relative mt-6 flex flex-col gap-4">
              {problems.map((p) => (
                <li key={p} className="flex gap-3 font-medium text-xl">
                  <span aria-hidden className="text-ember">
                    &rarr;
                  </span>
                  {p}
                </li>
              ))}
            </ul>
          </div>
        </section>

        <section aria-labelledby="prices" className="mx-auto max-w-6xl px-5 pb-20 md:px-6">
          <div className="mb-8 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
            <h2 id="prices" className="font-semibold text-3xl text-obsidian md:text-heading">
              Simple prices
            </h2>
            <Link
              href="/pricing"
              className="focus-ring w-fit rounded-badge text-iron underline underline-offset-4 hover:text-obsidian"
            >
              Yearly prices and how to pay
            </Link>
          </div>
          <Prices plans={plans} period="monthly" />
        </section>

        <section aria-labelledby="questions" className="mx-auto max-w-3xl px-5 pb-20 md:px-6">
          <h2 id="questions" className="mb-8 font-semibold text-3xl text-obsidian md:text-heading">
            Questions owners ask
          </h2>
          <Questions items={questions} />
          <p className="mt-6 text-steel">
            Something else?{" "}
            <Link href="/support" className="text-obsidian underline underline-offset-4">
              Ask us
            </Link>{" "}
            on WhatsApp at {SUPPORT.whatsapp} or at {SUPPORT.email}.
          </p>
        </section>

        <section
          id="download"
          aria-labelledby="download-title"
          className="mx-auto max-w-6xl scroll-mt-6 px-5 pb-24 md:px-6"
        >
          <div className="flex flex-col gap-6 rounded-card-lg border border-cloud bg-snow p-8 md:flex-row md:items-center md:justify-between md:p-12">
            <div className="flex max-w-xl flex-col gap-3">
              <h2 id="download-title" className="font-semibold text-3xl text-obsidian md:text-4xl">
                Ready when you are
              </h2>
              <p className="text-steel">
                Download StayKey, add your property and photos, and share your booking link before
                your next guest messages you. On a phone, staykey.direct/download opens the right
                store.
              </p>
            </div>
            <StoreButtons />
          </div>
        </section>
      </main>

      <SiteFooter />
    </>
  );
}
