import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Button, Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, KV, List, ListRow, SectionHeader, SwitchRow } from "@/components/kit";
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

/** Bank transfer details and deadline, pay at property, and card payments for owners who want them. */
export default function PaymentMethods() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, property } = useSettings(id, KEYS);
  const [editBank, setEditBank] = useState(false);
  if (!draft || !property)
    return (
      <SettingsPage title="Payment methods" missing>
        {null}
      </SettingsPage>
    );
  const pay = draft.payments;
  const bank = pay.bank;
  const setBank = (patch: Partial<typeof bank>) =>
    set({ payments: { ...pay, bank: { ...bank, ...patch } } });
  const none = !bank.enabled && !pay.atProperty && pay.cards !== "on";

  return (
    <SettingsPage title="Payment methods" dirty={dirty && !none} onSave={save}>
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
      <List>
        <ListRow
          icon={I.card}
          title="Card payments"
          subtitle={
            pay.cards === "pending"
              ? "Application in review"
              : pay.cards === "on"
                ? "Visa and Mastercard through PayHere"
                : "Approval takes a few days"
          }
          chevron={false}
          trailing={
            <Tag
              tone={pay.cards === "on" ? "spark" : pay.cards === "pending" ? "ember" : "soft"}
              label={pay.cards === "on" ? "On" : pay.cards === "pending" ? "In review" : "Off"}
            />
          }
        />
      </List>
      {pay.cards === "off" ? (
        <Button
          compact
          variant="ghost"
          title="Connect card payments"
          onPress={() => set({ payments: { ...pay, cards: "pending" } })}
        />
      ) : pay.cards === "pending" ? (
        <InfoNote icon={I.card}>
          We've started your PayHere application. You'll get an email with the documents they need,
          usually within a day. Save to keep this.
        </InfoNote>
      ) : null}
      {none ? (
        <InfoNote icon={I.warning}>Turn on at least one way for guests to pay.</InfoNote>
      ) : null}
    </SettingsPage>
  );
}
