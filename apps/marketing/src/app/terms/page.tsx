import type { Metadata } from "next";
import Link from "next/link";
import { Clause, LegalPage } from "@/components/legal-page";
import { COMPANY, LEGAL, LEGAL_REVIEWED, SUPPORT } from "@/lib/company";

export const metadata: Metadata = {
  title: "Terms of service",
  description:
    "The terms for using StayKey, the direct booking app for villas and guesthouses in Sri Lanka.",
  alternates: { canonical: "/terms" },
  robots: LEGAL_REVIEWED ? undefined : { index: false, follow: true },
};

export default function TermsPage() {
  return (
    <LegalPage
      title="Terms of service"
      version={LEGAL.terms.version}
      updated={LEGAL.terms.updated}
      intro={
        <p>
          These terms are an agreement between you and {COMPANY.name} ("StayKey", "we"), the company
          that runs StayKey. By creating an account or using StayKey you accept them. If you use
          StayKey for a business, you accept them for that business.
        </p>
      }
    >
      <Clause n={1} title="What StayKey does">
        <p>
          StayKey gives owners of villas, guesthouses and similar stays a booking page, a website
          widget, a calendar that syncs with other booking sites, tools to record payments, a team
          with roles, and messages to guests. Guests use booking pages to check dates and book.
        </p>
      </Clause>

      <Clause n={2} title="Your account">
        <ul>
          <li>You must be 18 or older and sign in with a phone number you control.</li>
          <li>Keep your details accurate, and keep your phone and sign-in codes to yourself.</li>
          <li>
            The account owner is responsible for everything done in the account, including by the
            managers and caretakers they invite, and for removing people who should no longer have
            access.
          </li>
        </ul>
      </Clause>

      <Clause n={3} title="Your property and your guests">
        <ul>
          <li>
            You are the host. Each booking is an agreement between you and your guest, on your
            prices, policies and house rules. StayKey is not a party to it and is not a travel
            agent.
          </li>
          <li>
            You are responsible for describing your property truthfully, honouring the bookings you
            accept, holding any registration or licence your property needs (for example with the
            Sri Lanka Tourism Development Authority), and paying the taxes on your income.
          </li>
          <li>
            You are responsible for how you use guests' personal data, as their controller under Sri
            Lanka's Personal Data Protection Act. Our <Link href="/privacy">privacy policy</Link>{" "}
            explains how we handle it for you.
          </li>
        </ul>
      </Clause>

      <Clause n={4} title="Plans and paying for StayKey">
        <ul>
          <li>
            StayKey is free until your first direct booking comes in, for up to 60 days. After that
            you need a plan; plans differ only by how many units guests can book.
          </li>
          <li>
            Plans bought in the app are subscriptions with the App Store or Google Play, priced and
            charged by them, and renew until you cancel them there. Their refunds follow the store's
            rules.
          </li>
          <li>
            Plans bought on the web are paid upfront in Sri Lankan rupees through Payments.lk for a
            month or a year, and don't renew. They aren't refundable, except where the law requires.
          </li>
          <li>
            If your free period or plan ends, you have 7 days to pay. After that your booking page
            stops taking bookings and asks guests to contact you on WhatsApp; the app and your data
            stay available, and paying again reopens the page.
          </li>
          <li>
            We may change prices. A change never affects a period you've already paid for, and we
            tell you at least 30 days before it applies to you.
          </li>
        </ul>
      </Clause>

      <Clause n={5} title="Guest payments">
        <ul>
          <li>
            Bank transfers and payments at the property go from the guest to you. StayKey only
            records them.
          </li>
          <li>
            Card payments go through your own Payments.lk merchant account, under your agreement
            with Payments.lk. StayKey sends guests to Payments.lk's page and records the result; it
            doesn't hold the money or add a charge.
          </li>
          <li>
            If StayKey offers to collect card payments for you and pay them out, that service has
            its own payout terms, which apply when you accept them in the app.
          </li>
          <li>Refunds to guests are your decision and your responsibility.</li>
        </ul>
      </Clause>

      <Clause n={6} title="Using StayKey fairly">
        <p>Don't use StayKey to:</p>
        <ul>
          <li>
            list a property you don't have the right to let, or take bookings you won't honour;
          </li>
          <li>mislead guests, or collect payments for anything other than real stays;</li>
          <li>send messages guests haven't asked for, or misuse their details;</li>
          <li>
            break the law, infringe others' rights, or try to break into, overload or copy StayKey.
          </li>
        </ul>
      </Clause>

      <Clause n={7} title="Your content and data">
        <p>
          Your photos, descriptions and data remain yours. You let us store, copy and show them as
          needed to run StayKey, for example on your booking page and in emails to your guests. You
          can download your data from the app at any time.
        </p>
        <p>
          StayKey itself, its software, design and name, belongs to us. You may not copy it or use
          our name except to tell people about your booking page.
        </p>
      </Clause>

      <Clause n={8} title="How StayKey works day to day">
        <p>
          We work to keep StayKey available and your data safe, but we can't promise it will never
          be interrupted. Calendar sync depends on other sites publishing their calendars; changes
          there can take a few minutes to reach StayKey, so keep an eye on bookings made close
          together. We may improve or change features, and we'll tell you before removing one you
          rely on.
        </p>
      </Clause>

      <Clause n={9} title="Liability">
        <p>
          As far as the law allows, StayKey is provided as it is, and we are not liable for indirect
          losses, lost profits or bookings, or disputes between you and your guests. Our total
          liability to you for any claim is limited to what you paid us in the 12 months before it.
          Nothing in these terms limits liability that the law doesn't allow us to limit.
        </p>
      </Clause>

      <Clause n={10} title="Ending">
        <p>
          You can delete your account at any time in the app; see{" "}
          <Link href="/delete-account">deleting your account</Link>. We may suspend or close an
          account that breaks these terms or puts guests or StayKey at risk, and will tell you why
          unless the law or safety prevents it.
        </p>
      </Clause>

      <Clause n={11} title="Law">
        <p>
          These terms are governed by the laws of Sri Lanka, and the courts of Colombo decide any
          dispute about them.
        </p>
      </Clause>

      <Clause n={12} title="Changes to these terms">
        <p>
          We'll update the version and date above when these terms change, and tell you in the app
          or by email at least 30 days before a change that affects you takes effect.
        </p>
      </Clause>

      <Clause n={13} title="Contact">
        <p>
          {COMPANY.name}, registration number {COMPANY.registration}, {COMPANY.address}.{" "}
          <a href={`mailto:${COMPANY.legalEmail}`}>{COMPANY.legalEmail}</a>, {SUPPORT.whatsapp}.
        </p>
      </Clause>
    </LegalPage>
  );
}
