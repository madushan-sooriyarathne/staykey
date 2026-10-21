import { router } from "expo-router";
import { messageFor } from "@/api/errors";
import { usePayouts } from "@/api/payouts";
import { Tag } from "@/components/brand";
import { InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, EmptyState, KV, List, ListRow, money, Page, SectionHeader } from "@/components/kit";
import { formatShort, toISO } from "@/data/dates";
import { useCan } from "@/data/hooks";
import { payoutStatusLabel } from "@/features/payouts/payouts";

/**
 * StayKey payouts: what is in the next payout, what waits for stays to start, and past payouts
 * with their statements. Amounts are rupees. Owners only.
 */
export default function PayoutsScreen() {
  const can = useCan();
  const owner = can("billing");
  const query = usePayouts();
  const p = query.data;

  if (!owner) {
    return (
      <Page title="Payouts">
        <InfoNote icon={I.lock}>Only the account owner can see payouts.</InfoNote>
      </Page>
    );
  }
  if (!p) {
    return (
      <Page title="Payouts">
        {query.error ? <InfoNote icon={I.warning}>{messageFor(query.error)}</InfoNote> : null}
      </Page>
    );
  }

  return (
    <Page title="Payouts">
      <Card>
        <KV label="Next payout" value={formatShort(p.nextPayoutOn)} />
        <KV label="In the next payout" value={money(Math.max(p.available, 0), "LKR")} />
        <KV label="Waiting for stays to start" value={money(p.pending, "LKR")} />
        {p.onTheWay > 0 ? (
          <KV label="On its way to your bank" value={money(p.onTheWay, "LKR")} />
        ) : null}
      </Card>
      {p.available < 0 ? (
        <InfoNote icon={I.warning}>
          Refunds took {money(-p.available, "LKR")} more than your balance. It comes out of your
          next payouts.
        </InfoNote>
      ) : null}
      {p.heldUntil ? (
        <InfoNote icon={I.clock}>
          Your bank account changed recently, so payouts are held until{" "}
          {formatShort(toISO(new Date(p.heldUntil)))}.
        </InfoNote>
      ) : null}
      <InfoNote icon={I.cash}>
        Payouts go every other Monday for stays that started by the Sunday before, less StayKey's 3%
        processing charge. Under {money(p.minimum, "LKR")} waits for the next one.
      </InfoNote>

      <SectionHeader title="Past payouts" />
      {p.payouts.length === 0 ? (
        <EmptyState icon={I.bank} title="No payouts yet" body="Your first payout shows here." />
      ) : (
        <List>
          {p.payouts.map((po) => (
            <ListRow
              key={po.id}
              testID={`payout-${po.id}`}
              icon={I.bank}
              title={money(po.amount, "LKR")}
              subtitle={`Card payments up to ${formatShort(po.periodEnd)}`}
              trailing={
                <Tag
                  tone={
                    po.status === "failed" ? "ember" : po.status === "paid" ? "spark" : "neutral"
                  }
                  label={payoutStatusLabel(po.status)}
                />
              }
              onPress={() => router.push({ pathname: "/payout/[id]", params: { id: po.id } })}
            />
          ))}
        </List>
      )}
    </Page>
  );
}
