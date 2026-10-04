import { colors, radius } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Button, Field, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import {
  Avatar,
  Card,
  Hint,
  List,
  ListRow,
  money,
  Page,
  SectionHeader,
  ui,
} from "@/components/kit";
import { font } from "@/components/ui";
import { formatClock, formatRange } from "@/data/dates";
import { useBooking, useProperty } from "@/data/hooks";
import { METHOD_LABEL } from "@/data/labels";
import { balanceOf } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Payment } from "@/data/types";
import { placeLabel } from "@/features/bookings/rows";
import { haptics } from "@/lib/haptics";

/** Logs a payment, and lets the owner accept or reject a bank slip the guest uploaded. */
export default function RecordPayment() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const booking = useBooking(id);
  const property = useProperty(booking?.propertyId);
  const record = useData((s) => s.recordPayment);
  const reject = useData((s) => s.rejectSlip);
  const slip = booking?.slip?.status === "pending" ? booking.slip : undefined;
  const balance = booking ? balanceOf(booking) : 0;
  const [amount, setAmount] = useState(
    ((slip?.amount ?? balance) / 100).toFixed(2).replace(/\.00$/, ""),
  );
  const [method, setMethod] = useState<Payment["method"]>(slip ? "bank" : "cash");

  if (!booking || !property) return null;
  const minor = Math.round(Number(amount || 0) * 100);
  const symbol = property.currency === "USD" ? "$" : "LKR";

  return (
    <Page
      title="Record payment"
      close
      footer={
        <>
          {slip ? (
            <View style={{ flex: 1 }}>
              <Button
                variant="ghost"
                title="Reject slip"
                onPress={() => {
                  haptics.warning();
                  reject(booking.id);
                  router.back();
                }}
              />
            </View>
          ) : null}
          <View style={{ flex: 1 }}>
            <Button
              testID="payment-record"
              title={`Record ${money(minor, property.currency)}`}
              disabled={minor <= 0}
              onPress={() => {
                haptics.success();
                record(booking.id, { amount: minor, method }, !!slip);
                router.back();
              }}
            />
          </View>
        </>
      }
    >
      <List>
        <ListRow
          leading={<Avatar name={booking.guest.name} />}
          title={booking.guest.name}
          subtitle={`${placeLabel(property, booking)}, ${formatRange(booking.checkIn, booking.checkOut)}`}
          trailing={
            <View style={{ alignItems: "flex-end" }}>
              <Text style={ui.faint}>Balance</Text>
              <Text style={ui.amount}>{money(balance, property.currency)}</Text>
            </View>
          }
        />
      </List>

      {slip ? (
        <Card title="Bank slip" meta={formatClock(slip.at)}>
          <View style={s.slip}>
            <SymbolView name={I.doc} tintColor={colors.steel} size={28} />
            <Text style={ui.faint}>
              Transfer slip from guest, {money(slip.amount, property.currency)}
            </Text>
          </View>
          <Hint>Check the money has reached your account before you accept the slip.</Hint>
        </Card>
      ) : null}

      <Field
        testID="payment-amount"
        label="Amount received"
        value={amount}
        onChangeText={(t) => setAmount(t.replace(/[^\d.]/g, ""))}
        keyboardType="decimal-pad"
        prefix={<Text style={s.prefix}>{symbol}</Text>}
      />
      <View style={{ gap: 8 }}>
        <SectionHeader title="Method" />
        <View style={s.pills}>
          {(["bank", "cash", "card"] as const).map((m) => (
            <Pill
              key={m}
              label={METHOD_LABEL[m]}
              selected={method === m}
              onPress={() => setMethod(m)}
            />
          ))}
        </View>
      </View>
      {minor > balance && balance > 0 ? (
        <Hint>That's more than the balance. The extra shows as overpaid on the booking.</Hint>
      ) : null}
    </Page>
  );
}

const s = StyleSheet.create({
  slip: {
    height: 150,
    borderRadius: radius.badge + 6,
    backgroundColor: colors.paper,
    alignItems: "center",
    justifyContent: "center",
    gap: 8,
    marginVertical: 6,
  },
  prefix: { fontFamily: font.medium, fontSize: 16, color: colors.steel },
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
