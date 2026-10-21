import type { Metadata } from "next";
import Link from "next/link";
import { Clause, LegalPage } from "@/components/legal-page";
import { COMPANY, LEGAL, SUPPORT } from "@/lib/company";
import { pageMetadata } from "@/lib/metadata";

// Not linked from the site and never indexed: the app links here from its payout details, which
// only show once StayKey payouts are offered.
export const metadata: Metadata = {
  ...pageMetadata({
    title: "Payout terms",
    description: "The terms for StayKey payouts, which aren't available yet.",
    path: "/terms/payouts",
  }),
  robots: { index: false, follow: false },
};

export default function PayoutTermsPage() {
  return (
    <LegalPage
      title="Payout terms"
      version={LEGAL.payouts.version}
      updated={LEGAL.payouts.updated}
      intro={
        <>
          <p className="font-medium text-obsidian">StayKey payouts aren't available yet.</p>
          <p className="mt-3">
            When they are, these terms apply to owners who choose them in the app, in addition to
            the <Link href="/terms">terms of service</Link>. With payouts, {COMPANY.name}{" "}
            ("StayKey") takes your guests' card payments on its own Payments.lk account and pays
            them on to your bank account.
          </p>
        </>
      }
    >
      <Clause n={1} title="Who can use payouts">
        <ul>
          <li>You must be 18 or older and hold a Sri Lankan NIC.</li>
          <li>
            Payouts go only to a bank account in Sri Lanka in your own name. We may ask for
            documents to confirm who you are and that the account is yours, and hold payouts until
            you provide them.
          </li>
          <li>You accept these terms, by their version, in the app's payout details.</li>
        </ul>
      </Clause>

      <Clause n={2} title="How guests pay">
        <p>
          Guests pay by card on Payments.lk's page, in Sri Lankan rupees. For a property priced in
          another currency, the amount is converted at the rate you set in the app, and your payouts
          are in rupees.
        </p>
      </Clause>

      <Clause n={3} title="Processing charge">
        <p>
          StayKey deducts a processing charge of 3% of each card payment, rounded up to the cent, to
          cover the cost of taking card payments. The charge isn't returned when a payment is
          refunded, because the payment providers keep their fees on refunds too. We tell you at
          least 30 days before the charge changes.
        </p>
      </Clause>

      <Clause n={4} title="When you're paid">
        <ul>
          <li>Money for a stay becomes payable the day after the guest checks in.</li>
          <li>
            Payouts are made every other Monday by bank transfer, for stays that started by the
            Sunday before, and usually reach your bank within 1 to 2 working days.
          </li>
          <li>A balance under Rs 1,000 waits for the next payout.</li>
          <li>
            Each payout comes with a statement in the app listing every stay, charge and refund.
          </li>
        </ul>
      </Clause>

      <Clause n={5} title="Refunds, chargebacks and negative balances">
        <ul>
          <li>
            Refunds you give guests, and chargebacks their banks raise, are taken from your balance.
          </li>
          <li>
            If they leave your balance below zero, later payouts make it up. If no payouts are due
            to make it up within 60 days, you agree to repay the difference when we ask.
          </li>
          <li>We may hold the money for a disputed payment until the dispute ends.</li>
        </ul>
      </Clause>

      <Clause n={6} title="Changing your bank account">
        <p>
          When you change the bank, branch or account number, the next payout is held for 72 hours
          and we email you and note it in Activity, so a change you didn't make can be stopped.
          Write to <a href={`mailto:${SUPPORT.email}`}>{SUPPORT.email}</a> straight away if you
          didn't make it.
        </p>
      </Clause>

      <Clause n={7} title="When we may hold payouts">
        <p>
          We may hold or delay a payout if your details are incomplete or wrong, if a payment looks
          fraudulent, if the law or a payment provider requires it, or if your account is closed. We
          tell you why unless the law prevents it.
        </p>
      </Clause>

      <Clause n={8} title="Records and tax">
        <p>
          Statements stay available in the app. You are responsible for the tax on your income from
          stays; the processing charge is StayKey's fee for its service.
        </p>
      </Clause>

      <Clause n={9} title="Ending payouts">
        <p>
          You can switch to your own Payments.lk account at any time. We pay any balance left, less
          refunds still to come, on the following payout days. We may end payouts with 30 days'
          notice, or straight away if these terms are broken.
        </p>
      </Clause>

      <Clause n={10} title="Changes and contact">
        <p>
          A new version of these terms applies once you accept it in the app. Questions:{" "}
          <a href={`mailto:${COMPANY.legalEmail}`}>{COMPANY.legalEmail}</a>, {SUPPORT.whatsapp}.
        </p>
      </Clause>
    </LegalPage>
  );
}
