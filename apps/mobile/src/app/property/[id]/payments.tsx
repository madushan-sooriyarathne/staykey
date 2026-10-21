import { colors } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { Button, Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, KV, List, SectionHeader, SwitchRow } from "@/components/kit";
import { font } from "@/components/ui";
import type { PropertyConfig } from "@/data/types";
import { SettingsPage, useSettings } from "@/features/property/settings";

const KEYS = ["payments"] as const;
const BANKS = [
  "Commercial Bank",
  "Sampath Bank",
  "Bank of Ceylon",
  "People's Bank",
  "Hatton National Bank",
  "Nations Trust Bank",
  "Seylan Bank",
  "DFCC Bank",
];

/** What the card switch says: ready, or the first thing keeping cards off. */
function cardsLine(pay: PropertyConfig["payments"], currency: string): string {
  if (pay.cards === "off") return "Guests pay through Payments.lk";
  switch (pay.cardsMissing?.[0]) {
    case "mode":
    case "connection":
      return "Waiting for your Payments.lk account";
    case "payout_profile":
      return "Waiting for your payout details";
    case "rate":
      return `Waiting for a rupee rate for ${currency}`;
  }
  return pay.cards === "on" ? "Guests can pay by card" : "Save to turn cards on";
}

/** Bank transfer details and deadline, pay at property, and card payments through Payments.lk. */
export default function PaymentMethods() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [editBank, setEditBank] = useState(false);
  // What the owner is typing into the rate, so "300." isn't reformatted under them.
  const [typedRate, setRateText] = useState<string | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Payment methods" missing>
        {null}
      </SettingsPage>
    );
  const pay = draft.payments;
  const rateText = typedRate ?? (pay.cardRate ? (pay.cardRate / 100).toFixed(2) : "");
  const bank = pay.bank;
  const setBank = (patch: Partial<typeof bank>) =>
    set({ payments: { ...pay, bank: { ...bank, ...patch } } });
  const none = !bank.enabled && !pay.atProperty && pay.cards === "off";

  return (
    <SettingsPage title="Payment methods" dirty={dirty && !none} onSave={save} status={status}>
      <Card>
        <SwitchRow
          icon={I.bank}
          title="Bank transfer"
          subtitle="Guests upload a slip and you confirm"
          value={bank.enabled}
          onChange={(enabled) => setBank({ enabled })}
        />
        {bank.enabled ? (
          <Animated.View
            entering={FadeInDown.duration(220)}
            exiting={FadeOut.duration(120)}
            style={{ gap: 4 }}
          >
            {editBank ? (
              <View style={{ gap: 10, paddingVertical: 8 }}>
                <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
                  {BANKS.map((b) => (
                    <Pill
                      key={b}
                      label={b}
                      selected={bank.bankName === b}
                      onPress={() => setBank({ bankName: b })}
                    />
                  ))}
                </View>
                <Field
                  label="Account name"
                  value={bank.accountName}
                  onChangeText={(accountName) => setBank({ accountName })}
                />
                <Field
                  label="Account number"
                  value={bank.accountNumber}
                  onChangeText={(t) =>
                    setBank({ accountNumber: t.replace(/\D/g, "").slice(0, 20) })
                  }
                  keyboardType="number-pad"
                />
                <Button compact variant="ghost" title="Done" onPress={() => setEditBank(false)} />
              </View>
            ) : (
              <>
                <KV label="Bank" value={bank.bankName || "Not set"} />
                <KV label="Account name" value={bank.accountName || "Not set"} />
                <KV
                  label="Account"
                  value={bank.accountNumber ? `Ending ${bank.accountNumber.slice(-4)}` : "Not set"}
                />
                <View style={{ paddingVertical: 6 }}>
                  <Button
                    compact
                    variant="ghost"
                    title="Edit bank details"
                    icon={I.edit}
                    onPress={() => setEditBank(true)}
                  />
                </View>
              </>
            )}
            <SectionHeader title="Guest pays within" />
            <View style={{ flexDirection: "row", gap: 8, paddingBottom: 4 }}>
              {[12, 24, 48].map((h) => (
                <Pill
                  key={h}
                  label={`${h} hours`}
                  selected={bank.payWithinHours === h}
                  onPress={() => setBank({ payWithinHours: h })}
                />
              ))}
            </View>
            <SwitchRow
              title="Cancel if unpaid"
              subtitle="Frees the dates when the deadline passes"
              value={bank.cancelIfUnpaid}
              onChange={(cancelIfUnpaid) => setBank({ cancelIfUnpaid })}
            />
          </Animated.View>
        ) : null}
      </Card>
      <List>
        <SwitchRow
          icon={I.cash}
          title="Pay at property"
          subtitle="Cash or card on arrival"
          value={pay.atProperty}
          onChange={(atProperty) => set({ payments: { ...pay, atProperty } })}
        />
      </List>
      <Card>
        <SwitchRow
          icon={I.card}
          title="Card payments"
          subtitle={cardsLine(pay, property.currency)}
          value={pay.cards !== "off"}
          onChange={(on) => set({ payments: { ...pay, cards: on ? "on" : "off" } })}
        />
        {pay.cards !== "off" ? (
          <Animated.View
            entering={FadeInDown.duration(220)}
            exiting={FadeOut.duration(120)}
            style={{ gap: 8, paddingBottom: 6 }}
          >
            <SectionHeader title="A card pays when booking" />
            <View style={{ flexDirection: "row", gap: 8 }}>
              <Pill
                label={
                  property.depositPercent > 0
                    ? `The deposit, ${property.depositPercent}%`
                    : "The deposit"
                }
                selected={pay.cardCharge === "deposit"}
                onPress={() => set({ payments: { ...pay, cardCharge: "deposit" } })}
              />
              <Pill
                label="The full amount"
                selected={pay.cardCharge === "full"}
                onPress={() => set({ payments: { ...pay, cardCharge: "full" } })}
              />
            </View>
            {property.currency !== "LKR" ? (
              <Field
                testID="cards-rate"
                label={`Rupees per ${property.currency}, for card checkouts`}
                value={rateText}
                onChangeText={(t) => {
                  const clean = t.replace(/[^\d.]/g, "");
                  setRateText(clean);
                  const cents = Math.round(Number(clean) * 100);
                  set({ payments: { ...pay, cardRate: cents > 0 ? cents : undefined } });
                }}
                keyboardType="decimal-pad"
                prefix={<Text style={s.prefix}>Rs</Text>}
              />
            ) : null}
          </Animated.View>
        ) : null}
      </Card>
      {pay.cards !== "off" && pay.cardsMissing?.length ? (
        <Button
          compact
          variant="ghost"
          title="Set up card payments"
          icon={I.card}
          onPress={() => router.push("/card-payments")}
        />
      ) : null}
      {none ? (
        <InfoNote icon={I.warning}>Turn on at least one way for guests to pay.</InfoNote>
      ) : null}
    </SettingsPage>
  );
}

const s = StyleSheet.create({
  prefix: { fontFamily: font.medium, fontSize: 16, color: colors.steel },
});
