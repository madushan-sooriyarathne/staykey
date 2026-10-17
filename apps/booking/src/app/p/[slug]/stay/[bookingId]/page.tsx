import { formatMoney, type GuestBooking } from "@staykey/api-client";
import type { Metadata } from "next";
import { notFound } from "next/navigation";
import { getGuestBooking, getPublicProperty } from "@/lib/api";
import { cx } from "@/lib/cx";
import { formatRange } from "@/lib/dates";
import { param } from "@/lib/stay-params";
import { SlipForm } from "./slip-form";

// The link carries the guest's token, so it must not leak to other sites or search engines.
export const metadata: Metadata = {
  title: "Your booking | StayKey",
  referrer: "no-referrer",
  robots: { index: false, follow: false },
};

const STATUS: Record<GuestBooking["status"], { title: string; note: string }> = {
  requested: { title: "Request sent", note: "The owner will confirm your dates soon." },
  awaiting_payment: { title: "Booked", note: "Your dates are held while we wait for payment." },
  confirmed: { title: "Confirmed", note: "You're all set. See you soon." },
  checked_in: { title: "Checked in", note: "Enjoy your stay." },
  checked_out: { title: "Checked out", note: "Thanks for staying." },
  cancelled: { title: "Cancelled", note: "This booking was cancelled." },
  declined: { title: "Not available", note: "The owner couldn't take this booking." },
};

export default async function StayPage(props: PageProps<"/p/[slug]/stay/[bookingId]">) {
  const { slug, bookingId } = await props.params;
  const token = param(await props.searchParams, "t");
  if (!token) notFound();
  const [b, property] = await Promise.all([
    getGuestBooking(bookingId, token),
    getPublicProperty(slug),
  ]);
  if (!b) notFound();
  const timeZone = property?.timeZone ?? "UTC";
  const status = STATUS[b.status];
  const deadline = b.status === "requested" ? b.requestExpiresAt : b.paymentDueAt;
  const takesSlips =
    b.paymentMethod === "bank" &&
    b.balance > 0 &&
    ["awaiting_payment", "confirmed", "checked_in"].includes(b.status);

  return (
    <main className="mx-auto flex max-w-xl flex-col gap-6 px-4 py-6 md:py-10">
      <header className="flex flex-col gap-1">
        <p className="text-sm text-steel">
          {b.propertyName} · {b.ref}
        </p>
        <h1 className="font-semibold text-3xl text-obsidian">{status.title}</h1>
        <p className="text-graphite">{status.note}</p>
        <p className="text-sm text-steel">
          Keep this page's link: it's how you come back to your booking.
        </p>
      </header>

      <section className="flex flex-col gap-1 rounded-card border border-cloud bg-snow p-5 text-graphite">
        <p className="font-medium text-obsidian">
          {formatRange(b.checkIn, b.checkOut)}, {b.unitName}
        </p>
        <p>
          {b.adults} {b.adults === 1 ? "adult" : "adults"}
          {b.children > 0 && `, ${b.children} ${b.children === 1 ? "child" : "children"}`}
        </p>
        <p className="text-sm text-steel">Check-in from {b.propertyCheckIn}</p>
      </section>

      <section className="flex flex-col gap-2 rounded-card bg-subtle p-4">
        {b.lines.map((l) => (
          <p key={l.label} className="flex justify-between text-graphite">
            <span>{l.label}</span>
            <span>{formatMoney(l.amount, b.currency)}</span>
          </p>
        ))}
        <Row label="Total" value={formatMoney(b.total, b.currency)} strong />
        {b.paid > 0 && <Row label="Paid" value={formatMoney(b.paid, b.currency)} />}
        {b.balance > 0 && b.balance !== b.total && (
          <Row label="Left to pay" value={formatMoney(b.balance, b.currency)} />
        )}
        {b.deposit > 0 && b.paid === 0 && b.deposit < b.total && (
          <Row label="Deposit to secure it" value={formatMoney(b.deposit, b.currency)} />
        )}
      </section>

      {deadline && (
        <p className="rounded-card bg-ember-tint p-4 text-ember-ink">
          {b.status === "requested" ? "The owner answers by" : "Please pay by"}{" "}
          {new Date(deadline).toLocaleString("en-GB", {
            dateStyle: "medium",
            timeStyle: "short",
            timeZone,
          })}
          {b.status === "requested" ? "." : ", or the booking is cancelled."}
        </p>
      )}

      {b.paymentMethod === "property" && b.status !== "cancelled" && b.status !== "declined" && (
        <p className="text-graphite">You pay at the property when you arrive.</p>
      )}

      {b.bank && b.status !== "requested" && (
        <section className="flex flex-col gap-2 rounded-card border border-cloud bg-snow p-5">
          <h2 className="font-semibold text-obsidian text-xl">Bank transfer</h2>
          <dl className="grid grid-cols-[auto_1fr] gap-x-6 gap-y-1 text-graphite">
            <dt className="text-steel">Bank</dt>
            <dd>{b.bank.bankName}</dd>
            <dt className="text-steel">Account name</dt>
            <dd>{b.bank.accountName}</dd>
            <dt className="text-steel">Account number</dt>
            <dd className="font-medium">{b.bank.accountNumber}</dd>
            <dt className="text-steel">Reference</dt>
            <dd className="font-medium">{b.ref}</dd>
          </dl>
        </section>
      )}

      {b.slips.length > 0 && (
        <section className="flex flex-col gap-2">
          <h2 className="font-semibold text-obsidian text-xl">Slips you sent</h2>
          <ul className="flex flex-col gap-1 text-graphite">
            {b.slips.map((s) => (
              <li key={s.uploadedAt} className="flex justify-between">
                <span>{formatMoney(s.amount, b.currency)}</span>
                <span className="text-steel">{SLIP_STATUS[s.status]}</span>
              </li>
            ))}
          </ul>
        </section>
      )}

      {takesSlips && <SlipForm bookingId={b.id} token={token} currency={b.currency} />}
    </main>
  );
}

const SLIP_STATUS: Record<GuestBooking["slips"][number]["status"], string> = {
  pending: "Waiting for the owner to check",
  accepted: "Received",
  rejected: "Not accepted, please send another",
};

function Row({ label, value, strong }: { label: string; value: string; strong?: boolean }) {
  return (
    <p
      className={cx(
        "flex justify-between",
        strong ? "border-cloud border-t pt-2 font-semibold text-obsidian" : "text-sm text-steel",
      )}
    >
      <span>{label}</span>
      <span>{value}</span>
    </p>
  );
}
