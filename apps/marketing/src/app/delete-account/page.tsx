import type { Metadata } from "next";
import Link from "next/link";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { SUPPORT } from "@/lib/company";
import { pageMetadata } from "@/lib/metadata";

export const metadata: Metadata = pageMetadata({
  title: "Delete your account",
  description:
    "How to delete your StayKey account and data, what is deleted, and how to ask us if you can't open the app.",
  path: "/delete-account",
});

const requestMail = `mailto:${SUPPORT.email}?subject=${encodeURIComponent(
  "Delete my StayKey account",
)}&body=${encodeURIComponent(
  "Please delete my StayKey account.\n\nThe phone number I sign in with: \nMy name: \n",
)}`;

export default function DeleteAccountPage() {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-5 pt-10 pb-20 md:px-6 md:pt-16">
        <h1 className="font-semibold text-4xl text-obsidian leading-tight md:text-5xl">
          Delete your StayKey account
        </h1>
        <p className="mt-4 text-body-lg text-steel">
          You can delete your account and your data whenever you like, from the app.
        </p>

        <section aria-labelledby="in-app" className="mt-12 flex flex-col gap-4">
          <h2 id="in-app" className="font-semibold text-2xl text-obsidian">
            In the app
          </h2>
          <ol className="flex list-decimal flex-col gap-2 pl-5 text-graphite">
            <li>Open StayKey and go to More, then Profile and account.</li>
            <li>Tap Delete account, then type DELETE to confirm.</li>
            <li>
              You're signed out everywhere and your booking pages go offline straight away. We email
              you to confirm, if your email is confirmed.
            </li>
          </ol>
          <p className="text-steel">
            Changed your mind? Sign in again with the same phone number within 30 days and
            everything comes back as it was.
          </p>
        </section>

        <section aria-labelledby="what" className="mt-12 flex flex-col gap-4">
          <h2 id="what" className="font-semibold text-2xl text-obsidian">
            What is deleted
          </h2>
          <ul className="flex list-disc flex-col gap-2 pl-5 text-graphite">
            <li>
              After 30 days: your profile, the accounts you own with their properties, photos,
              bookings, guests, payments, messages and team, and your sign-ins and devices.
            </li>
            <li>
              If you were a manager or caretaker in someone else's account, you're removed from it.
              The bookings you worked on stay in the owner's account.
            </li>
            <li>
              Records the law requires us to keep, such as payment and payout records, are kept for
              as long as Sri Lankan tax law requires, and then deleted. Backups roll off shortly
              after.
            </li>
          </ul>
          <p className="text-steel">
            A plan bought in the App Store or Google Play keeps renewing until you cancel it there,
            so the app asks you to cancel it first. Want a copy of your data before you go? Use
            Download your data in the same screen.
          </p>
        </section>

        <section aria-labelledby="ask" className="mt-12 flex flex-col gap-4">
          <h2 id="ask" className="font-semibold text-2xl text-obsidian">
            Can't open the app?
          </h2>
          <p className="text-graphite">
            <a
              href={requestMail}
              className="focus-ring rounded-badge text-obsidian underline underline-offset-4"
            >
              Email {SUPPORT.email}
            </a>{" "}
            or message us on WhatsApp at {SUPPORT.whatsapp}, from the phone number you sign in with
            if you can. We confirm it's you, delete the account the same way, and reply when it's
            done.
          </p>
          <p className="text-steel">
            Guests who booked a stay through StayKey can ask the property's owner to remove their
            details, or write to us. See the{" "}
            <Link href="/privacy" className="text-obsidian underline underline-offset-4">
              privacy policy
            </Link>
            .
          </p>
        </section>
      </main>
      <SiteFooter />
    </>
  );
}
