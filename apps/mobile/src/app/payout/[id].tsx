import { useLocalSearchParams } from "expo-router";
import { messageFor } from "@/api/errors";
import { usePayout } from "@/api/payouts";
import { InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, KV, List, ListRow, money, Page, SectionHeader } from "@/components/kit";
import { formatShort, toISO } from "@/data/dates";
import { entryLabel, payoutStatusLabel } from "@/features/payouts/payouts";

/** A payout's statement: each stay's card payment, StayKey's charge, refunds and adjustments. */
export default function PayoutStatementScreen() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const query = usePayout(id ?? "");
  const st = query.data;

  if (!st) {
    return (
      <Page title="Payout">
        {query.error ? <InfoNote icon={I.warning}>{messageFor(query.error)}</InfoNote> : null}
      </Page>
    );
  }
  const { payout, lines } = st;

  return (
    <Page title="Payout">
      <Card>
        <KV label="Amount" value={money(payout.amount, "LKR")} />
        <KV label="Status" value={payoutStatusLabel(payout.status)} />
        <KV label="Covers card payments up to" value={formatShort(payout.periodEnd)} />
        {payout.paidAt ? (
          <KV label="Paid" value={formatShort(toISO(new Date(payout.paidAt)))} />
        ) : null}
      </Card>
      {payout.status === "failed" ? (
        <InfoNote icon={I.warning}>
          This payout didn't reach your bank{payout.reason ? ` (${payout.reason})` : ""}. The money
          is back in your balance for the next payout.
        </InfoNote>
      ) : null}

      <SectionHeader title="Statement" />
      <List>
        {lines.map((l) => (
          <ListRow
            key={l.id}
            icon={l.amount < 0 ? I.arrowOut : I.arrowIn}
            title={entryLabel(l.kind)}
            subtitle={[l.bookingRef, l.guestName, l.kind === "adjustment" ? l.note : null]
              .filter(Boolean)
              .join(", ")}
            value={money(l.amount, "LKR")}
          />
        ))}
      </List>
    </Page>
  );
}
