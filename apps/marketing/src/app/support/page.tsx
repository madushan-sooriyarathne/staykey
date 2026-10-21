import type { Metadata } from "next";
import Link from "next/link";
import { type Question, Questions } from "@/components/questions";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { SUPPORT } from "@/lib/company";
import { pageMetadata } from "@/lib/metadata";

export const metadata: Metadata = pageMetadata({
  title: "Help and support",
  description:
    "Help with StayKey: connect Airbnb, add the widget, take card payments, invite your team. Reach us on WhatsApp or by email.",
  path: "/support",
});

const topics: Question[] = [
  {
    q: "Connect your Airbnb calendar",
    a: "In Airbnb, open Calendar, then Availability, then Connect calendars, and copy the export link. In StayKey, open your property, then iCal sync, and paste it. Then copy your StayKey link from the same screen into Airbnb, so Airbnb closes the dates booked through StayKey. Booking.com works the same way.",
  },
  {
    q: "Add the widget to your website",
    a: "Open your property, then Share and embed. Copy the widget code and paste it where the booking box should appear, or tap Email developer to send it to whoever looks after your site.",
  },
  {
    q: "Record a WhatsApp booking",
    a: "Tap the plus on Today, pick the dates and fill in the guest. The price comes from your rates and you can change it. Record a deposit straight away if the guest has already paid something.",
  },
  {
    q: "Take card payments",
    a: "Open More, then Card payments, and connect your Payments.lk account by pasting the secret key from its dashboard. Payments.lk needs a business registered in Sri Lanka. Then turn on cards in each property's payment methods. Guests pay in rupees, and the money goes straight to your Payments.lk account.",
  },
  {
    q: "Invite a caretaker or manager",
    a: "Go to More, then Team, and tap Invite. Enter their WhatsApp number and pick a role. Caretakers see arrivals, notes and guest contact details, but never prices or payments. Managers run bookings and settings with you.",
  },
  {
    q: "Change or cancel your plan",
    a: (
      <>
        See plans in the app under More, then Subscription. A plan bought in the app is cancelled in
        the App Store or Google Play; a plan bought on the web is paid upfront and simply ends. More
        on{" "}
        <Link href="/pricing" className="text-obsidian underline underline-offset-4">
          pricing
        </Link>
        .
      </>
    ),
  },
  {
    q: "Delete your account",
    a: (
      <>
        Open Profile and tap Delete account. You have 30 days to change your mind by signing in
        again.{" "}
        <Link href="/delete-account" className="text-obsidian underline underline-offset-4">
          What is deleted
        </Link>
        .
      </>
    ),
  },
];

export default function SupportPage() {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-6xl px-5 pt-10 pb-20 md:px-6 md:pt-16">
        <div className="flex max-w-3xl flex-col gap-4">
          <h1 className="font-semibold text-5xl text-obsidian leading-[1.08] md:text-heading-lg">
            How can we help?
          </h1>
          <p className="text-body-lg text-steel">
            Message us and a person who knows StayKey replies, usually within one working day, Sri
            Lanka time.
          </p>
        </div>

        <div className="mt-10 grid gap-4 md:grid-cols-2">
          <a
            href={SUPPORT.whatsappLink}
            className="focus-ring flex flex-col gap-2 rounded-card-lg border border-cloud bg-snow p-7 hover:border-mist md:p-8"
          >
            <span className="font-semibold text-obsidian text-xl">WhatsApp</span>
            <span className="text-steel">{SUPPORT.whatsapp}</span>
            <span className="mt-2 text-iron text-sm underline underline-offset-4">Open a chat</span>
          </a>
          <a
            href={`mailto:${SUPPORT.email}?subject=${encodeURIComponent("Help with StayKey")}`}
            className="focus-ring flex flex-col gap-2 rounded-card-lg border border-cloud bg-snow p-7 hover:border-mist md:p-8"
          >
            <span className="font-semibold text-obsidian text-xl">Email</span>
            <span className="text-steel">{SUPPORT.email}</span>
            <span className="mt-2 text-iron text-sm underline underline-offset-4">Write to us</span>
          </a>
        </div>
        <p className="mt-4 text-steel text-sm">
          Include the phone number you sign in with, so we can find your account. We never ask for
          your sign-in code.
        </p>

        <section aria-labelledby="topics" className="mt-20 max-w-3xl">
          <h2 id="topics" className="mb-8 font-semibold text-3xl text-obsidian md:text-heading">
            Common questions
          </h2>
          <Questions items={topics} />
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
