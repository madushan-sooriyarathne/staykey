import type { Metadata } from "next";
import Link from "next/link";
import { Clause, LegalPage } from "@/components/legal-page";
import { COMPANY, LEGAL, LEGAL_REVIEWED, SUPPORT } from "@/lib/company";
import { pageMetadata } from "@/lib/metadata";

export const metadata: Metadata = pageMetadata({
  title: "Privacy policy",
  description:
    "What StayKey collects about owners, their teams and their guests, why, who it is shared with, and your rights under Sri Lanka's Personal Data Protection Act.",
  path: "/privacy",
  index: LEGAL_REVIEWED,
});

export default function PrivacyPage() {
  return (
    <LegalPage
      title="Privacy policy"
      version={LEGAL.privacy.version}
      updated={LEGAL.privacy.updated}
      intro={
        <p>
          StayKey helps villa and guesthouse owners take direct bookings. This policy explains what
          personal data StayKey handles, why, who sees it and what you can ask us to do with it,
          under Sri Lanka's Personal Data Protection Act, No. 9 of 2022.
        </p>
      }
    >
      <Clause n={1} title="Who we are">
        <p>
          StayKey is run by {COMPANY.name} (registration number {COMPANY.registration}),{" "}
          {COMPANY.address}. For anything about your personal data, write to{" "}
          <a href={`mailto:${COMPANY.legalEmail}`}>{COMPANY.legalEmail}</a>.
        </p>
        <p>
          We decide how and why personal data about owners and their team members is used, so we are
          its controller. Data about guests belongs to the owner whose property they book: the owner
          is its controller, and we handle it on the owner's behalf to run their booking page,
          calendar and messages.
        </p>
      </Clause>

      <Clause n={2} title="What we collect">
        <p>From owners and team members who use the app:</p>
        <ul>
          <li>your phone number, name, and, if you add them, your email address and photo;</li>
          <li>
            the names of the devices you sign in on and their notification tokens, so we can send
            alerts and tell you about a sign-in on a new device;
          </li>
          <li>
            your properties, rates, rules, photos and settings, and the bookings, payments, messages
            and activity in your account;
          </li>
          <li>
            the bank details guests pay into, and, if you connect one, your Payments.lk secret key,
            both stored encrypted;
          </li>
          <li>
            if StayKey payouts are offered and you use them, your legal name, NIC number and the
            bank account payouts go to, with the NIC and account number stored encrypted;
          </li>
          <li>
            your plan and payment history. Card details for plans are handled by the App Store,
            Google Play or Payments.lk, never by us.
          </li>
        </ul>
        <p>From guests who book through a StayKey booking page or widget:</p>
        <ul>
          <li>your name, phone number, and, if you give them, your email address and country;</li>
          <li>your dates, guests, extras, notes to the host, and the bank slip you upload;</li>
          <li>
            records of card payments and refunds. Card numbers are entered on Payments.lk's page and
            never reach StayKey.
          </li>
        </ul>
        <p>
          From visitors to this website: our servers record addresses and requests for security and
          to limit abuse. We don't use analytics, advertising trackers or tracking cookies. The web
          billing page sets cookies only to keep you signed in.
        </p>
      </Clause>

      <Clause n={3} title="Why we use it">
        <ul>
          <li>
            To provide StayKey: signing you in, running booking pages, calendars, payments and
            messages, and sending the alerts and emails you choose. This is needed to perform our
            agreement with you.
          </li>
          <li>
            To keep StayKey safe: limiting sign-in attempts, spotting abuse and fraud, and keeping
            records of changes to bookings and payments. These are our legitimate interests.
          </li>
          <li>To meet our obligations under Sri Lankan law, such as tax and accounting records.</li>
        </ul>
        <p>We don't sell personal data, and we don't send marketing to guests.</p>
      </Clause>

      <Clause n={4} title="Guests' data and owners">
        <p>
          Owners see the details of guests who book their property, and decide what to do with them.
          If you stayed at, or booked, a property through StayKey, the owner is the first person to
          ask about your data. You can also write to us and we'll pass your request on or help the
          owner answer it. Owners can remove a guest's details from their account at any time.
        </p>
      </Clause>

      <Clause n={5} title="Who we share it with">
        <p>We use these providers to run StayKey, each only for its job and under contract:</p>
        <ul>
          <li>our hosting and database provider, which runs the StayKey servers;</li>
          <li>Cloudflare, for storing photos, logos and bank slips;</li>
          <li>Amazon Web Services, for sending email;</li>
          <li>oZone, for sending sign-in codes by SMS;</li>
          <li>Expo, for delivering app notifications;</li>
          <li>Meta, for WhatsApp alerts, once they are switched on;</li>
          <li>Payments.lk and Payable, for card payments and refunds;</li>
          <li>RevenueCat, Apple and Google, for plans bought in the app.</li>
        </ul>
        <p>
          We may also share data when the law requires it, or to protect people from harm or fraud.
        </p>
      </Clause>

      <Clause n={6} title="Data outside Sri Lanka">
        <p>
          Some of these providers store or process data outside Sri Lanka, for example in Singapore,
          the European Union and the United States. We choose providers that protect personal data
          to a standard the Act accepts, and bind them by contract to use it only for StayKey.
        </p>
      </Clause>

      <Clause n={7} title="How long we keep it">
        <ul>
          <li>
            Your account data is kept while you use StayKey. When you delete your account, it is
            closed straight away and deleted for good after 30 days, unless you sign in again before
            then.
          </li>
          <li>
            Guests' details stay in the owner's account until the owner removes them or deletes the
            account. Bookings and payments are kept as the owner's records.
          </li>
          <li>
            Records we must keep by law, such as payments and payouts, are kept for as long as Sri
            Lankan tax and accounting law requires.
          </li>
          <li>Backups are kept for a short period and then overwritten.</li>
        </ul>
      </Clause>

      <Clause n={8} title="How we protect it">
        <p>
          Sign-in uses one-time codes sent to your phone. Bank details, payment keys and NIC numbers
          are encrypted when stored. Each account's data is kept apart from every other account's,
          and only the few people who run StayKey can reach the systems, for support and to keep
          them running.
        </p>
      </Clause>

      <Clause n={9} title="Your rights">
        <p>Under the Act you can ask us to:</p>
        <ul>
          <li>tell you what personal data we hold about you and give you a copy;</li>
          <li>correct data that is wrong or incomplete;</li>
          <li>delete your data, or stop using it, where the law allows;</li>
          <li>withdraw consent you gave, where we rely on it;</li>
          <li>review a decision about you made only by automated means. StayKey makes none.</li>
        </ul>
        <p>
          Owners can see, change, download and delete most of their data in the app themselves (see{" "}
          <Link href="/delete-account">deleting your account</Link>). For anything else, write to{" "}
          <a href={`mailto:${COMPANY.legalEmail}`}>{COMPANY.legalEmail}</a> from the email or phone
          number on your account. We reply within the time the Act sets. If you're not satisfied,
          you can complain to the Data Protection Authority of Sri Lanka.
        </p>
      </Clause>

      <Clause n={10} title="Children">
        <p>
          StayKey is for property owners and their teams, who must be 18 or older. Guests under 18
          should book with a parent or guardian.
        </p>
      </Clause>

      <Clause n={11} title="Changes">
        <p>
          When we change this policy, we update the version and date above, and tell owners in the
          app or by email before a significant change takes effect.
        </p>
      </Clause>

      <Clause n={12} title="Contact">
        <p>
          {COMPANY.name}, {COMPANY.address}. Data protection:{" "}
          <a href={`mailto:${COMPANY.legalEmail}`}>{COMPANY.legalEmail}</a>, {SUPPORT.whatsapp}.
          Everyday help: <a href={`mailto:${SUPPORT.email}`}>{SUPPORT.email}</a>.
        </p>
      </Clause>
    </LegalPage>
  );
}
