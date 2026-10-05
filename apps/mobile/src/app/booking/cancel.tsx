import { colors } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { useCancelBooking } from "@/api/bookings";
import { messageFor } from "@/api/errors";
import { Button, Field, InfoNote, Pill, Radio } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, Card, List, ListRow, money, Page, SwitchRow, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { formatRange } from "@/data/dates";
import { uid } from "@/data/defaults";
import { useBooking, useProperty } from "@/data/hooks";
import { plural } from "@/data/labels";
import { POLICY_TEXT, paidOf, suggestedRefund } from "@/data/pricing";
import { useData } from "@/data/store";
import { placeLabel } from "@/features/bookings/rows";
import { haptics } from "@/lib/haptics";

const REASONS = [
  "Guest changed plans",
  "I need the dates",
  "Guest stopped replying",
  "Something else",
];

/** Suggests a refund from the cancellation policy, records a reason and frees the dates. */
export default function CancelBooking() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const booking = useBooking(id);
  const property = useProperty(booking?.propertyId);
  const cancel = useCancelBooking();
  const log = useData((s) => s.log);
  const [key] = useState(() => uid("cancel"));
  const [error, setError] = useState<string | null>(null);
  const suggestion =
    booking && property
      ? suggestedRefund(property.policy, booking)
      : { amount: 0, share: 0, daysOut: 0 };
  const [refund, setRefund] = useState((suggestion.amount / 100).toFixed(2).replace(/\.00$/, ""));
  const [reason, setReason] = useState(REASONS[0] as string);
  const [notify, setNotify] = useState(true);

  if (!booking || !property) return null;
  const paid = paidOf(booking);
  const minor = Math.round(Number(refund || 0) * 100);
  const first = booking.guest.name.split(" ")[0];
  const symbol = property.currency === "USD" ? "$" : "LKR";
  const set = (n: number) => setRefund((n / 100).toFixed(2).replace(/\.00$/, ""));
  // Refunds go back the way the guest last paid.
  const paidBy = booking.payments.findLast((p) => p.method !== "refund")?.method;
  const method = paidBy && paidBy !== "refund" ? paidBy : "bank";

  async function confirm() {
    if (!booking) return;
    haptics.warning();
    setError(null);
    try {
      await cancel.mutateAsync({
        id: booking.id,
        key,
        version: booking.version,
        reason,
        refund: minor > 0 ? { amount: minor, method } : undefined,
      });
      log({
        kind: "cancellation",
        title: `${booking.guest.name}'s booking cancelled`,
        subtitle: `${formatRange(booking.checkIn, booking.checkOut)}, dates are open again`,
        bookingId: booking.id,
        propertyId: booking.propertyId,
      });
      router.back();
    } catch (e) {
      haptics.error();
      setError(messageFor(e));
    }
  }

  return (
    <Page
      title="Cancel booking"
      close
      footer={
        <>
          <View style={{ flex: 1 }}>
            <Button variant="ghost" title="Keep booking" onPress={() => router.back()} />
          </View>
          <View style={{ flex: 1 }}>
            <Button
              testID="cancel-confirm"
              title="Cancel booking"
              disabled={minor > paid || cancel.isPending}
              onPress={confirm}
            />
          </View>
        </>
      }
    >
      {error ? (
        <InfoNote icon={I.warning} testID="cancel-error">
          {error}
        </InfoNote>
      ) : null}
      <List>
        <ListRow
          leading={<Avatar name={booking.guest.name} />}
          title={booking.guest.name}
          subtitle={`${placeLabel(property, booking)}, ${formatRange(booking.checkIn, booking.checkOut)}`}
          trailing={
            <View style={{ alignItems: "flex-end" }}>
              <Text style={ui.faint}>Paid</Text>
              <Text style={ui.amount}>{money(paid, property.currency)}</Text>
            </View>
          }
        />
      </List>

      <Card title="Refund">
        <Text style={ui.muted}>
          {POLICY_TEXT[property.policy].label} policy:{" "}
          {POLICY_TEXT[property.policy].summary.toLowerCase()}.{" "}
          {suggestion.daysOut >= 0
            ? `${first} arrives in ${plural(suggestion.daysOut, "day")}.`
            : `${first} has already arrived.`}
        </Text>
        <View style={{ paddingTop: 10, gap: 10 }}>
          <Field
            testID="cancel-refund"
            value={refund}
            onChangeText={(t) => setRefund(t.replace(/[^\d.]/g, ""))}
            keyboardType="decimal-pad"
            prefix={<Text style={s.prefix}>{symbol}</Text>}
            error={minor > paid ? `You can refund up to ${money(paid, property.currency)}.` : null}
          />
          <View style={s.pills}>
            <Pill
              label="Full refund"
              selected={minor === paid && paid > 0}
              onPress={() => set(paid)}
            />
            {suggestion.share > 0 && suggestion.share < 1 ? (
              <Pill
                label={`Policy, ${Math.round(suggestion.share * 100)}%`}
                selected={minor === suggestion.amount}
                onPress={() => set(suggestion.amount)}
              />
            ) : null}
            <Pill label="No refund" selected={minor === 0} onPress={() => set(0)} />
          </View>
        </View>
      </Card>

      <List title="Reason">
        {REASONS.map((r) => (
          <ListRow
            key={r}
            leading={<Radio on={reason === r} />}
            title={r}
            chevron={false}
            onPress={() => setReason(r)}
          />
        ))}
      </List>

      {booking.guest.email || booking.guest.phone ? (
        <List>
          <SwitchRow
            title={`Tell ${first} about the cancellation`}
            subtitle={booking.guest.email ? "By email" : "By WhatsApp"}
            value={notify}
            onChange={setNotify}
          />
        </List>
      ) : null}
    </Page>
  );
}

const s = StyleSheet.create({
  prefix: { fontFamily: font.medium, fontSize: 16, color: colors.steel },
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
