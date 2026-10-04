import { colors } from "@staykey/tokens";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import Animated, { FadeIn, FadeInDown, FadeOut, LinearTransition } from "react-native-reanimated";
import {
  Card,
  styles as c,
  Divider,
  Field,
  OptionRow,
  Pill,
  SectionLabel,
  ToggleRow,
} from "@/components/controls";
import { Chip, font } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { formatTime, useOnboarding } from "../store";

const layout = LinearTransition.springify().damping(24).stiffness(220);

const CHECK_IN = ["12:00", "13:00", "14:00", "15:00"];
const CHECK_OUT = ["10:00", "11:00", "12:00"];

export function PriceStep() {
  const { draft, update } = useOnboarding();
  const symbol = draft.currency === "LKR" ? "Rs" : "$";

  return (
    <View style={{ gap: 14 }}>
      <View style={s.price}>
        <View style={s.priceTop}>
          <Text style={c.muted}>Price per night</Text>
          <View style={{ flexDirection: "row", gap: 6 }}>
            {(["USD", "LKR"] as const).map((cur) => (
              <Pill
                key={cur}
                label={cur}
                selected={draft.currency === cur}
                onPress={() => update({ currency: cur })}
              />
            ))}
          </View>
        </View>
        <View style={s.amountRow}>
          <Text style={s.symbol}>{symbol}</Text>
          <TextInput
            testID="nightly-rate"
            autoFocus
            keyboardType="decimal-pad"
            placeholder="0"
            placeholderTextColor={colors.mist}
            value={draft.nightlyRate}
            onChangeText={(t) => update({ nightlyRate: t.replace(/[^\d.]/g, "").slice(0, 9) })}
            style={s.amount}
            selectionColor={colors.ember}
          />
        </View>
      </View>

      <Animated.View layout={layout}>
        <Card>
          <ToggleRow
            label="Charge more on Friday and Saturday"
            value={draft.weekendOn}
            onChange={(weekendOn) => update({ weekendOn })}
          />
          {draft.weekendOn ? (
            <Animated.View
              entering={FadeInDown.duration(220)}
              exiting={FadeOut.duration(120)}
              style={{ paddingBottom: 12 }}
            >
              <Field
                keyboardType="decimal-pad"
                placeholder="Weekend price"
                value={draft.weekendRate}
                onChangeText={(t) => update({ weekendRate: t.replace(/[^\d.]/g, "") })}
                prefix={<Text style={c.muted}>Weekend {symbol}</Text>}
              />
            </Animated.View>
          ) : null}
        </Card>
      </Animated.View>

      <Animated.View layout={layout} style={{ gap: 8 }}>
        <SectionLabel>Check-in from</SectionLabel>
        <View style={s.pills}>
          {CHECK_IN.map((t) => (
            <Pill
              key={t}
              label={formatTime(t)}
              selected={draft.checkIn === t}
              onPress={() => update({ checkIn: t })}
            />
          ))}
        </View>
        <SectionLabel>Check-out by</SectionLabel>
        <View style={s.pills}>
          {CHECK_OUT.map((t) => (
            <Pill
              key={t}
              label={formatTime(t)}
              selected={draft.checkOut === t}
              onPress={() => update({ checkOut: t })}
            />
          ))}
        </View>
      </Animated.View>
    </View>
  );
}

const BANKS = [
  "Bank of Ceylon",
  "Commercial Bank",
  "DFCC Bank",
  "Hatton National Bank",
  "Nations Trust Bank",
  "NDB Bank",
  "Pan Asia Bank",
  "People's Bank",
  "Sampath Bank",
  "Seylan Bank",
];

const DEPOSITS = [0, 30, 50, 100];

export function PaymentStep() {
  const { draft, update } = useOnboarding();
  const [pickingBank, setPickingBank] = useState(false);

  return (
    <View style={{ gap: 12 }}>
      <Animated.View layout={layout}>
        <Card style={{ paddingBottom: draft.bankOn ? 16 : 6 }}>
          <ToggleRow
            label="Bank transfer"
            hint="Guests upload a slip and you confirm"
            value={draft.bankOn}
            onChange={(bankOn) => update({ bankOn })}
          />
          {draft.bankOn ? (
            <Animated.View
              entering={FadeInDown.duration(220)}
              exiting={FadeOut.duration(120)}
              style={{ gap: 10 }}
            >
              <Pressable
                accessibilityRole="button"
                onPress={() => {
                  haptics.select();
                  setPickingBank((v) => !v);
                }}
                style={[c.inputBox, pickingBank && c.inputFocused]}
              >
                <Text
                  style={[
                    c.input,
                    { paddingVertical: 15 },
                    !draft.bankName && { color: colors.ash },
                  ]}
                >
                  {draft.bankName || "Choose your bank"}
                </Text>
                <SymbolView
                  name={{
                    ios: pickingBank ? "chevron.up" : "chevron.down",
                    android: "expand_more",
                    web: "expand_more",
                  }}
                  tintColor={colors.fog}
                  size={14}
                />
              </Pressable>
              {pickingBank ? (
                <Animated.View
                  entering={FadeIn.duration(160)}
                  exiting={FadeOut.duration(100)}
                  style={s.bankList}
                >
                  {BANKS.map((bank) => (
                    <Pressable
                      key={bank}
                      onPress={() => {
                        haptics.select();
                        update({ bankName: bank });
                        setPickingBank(false);
                      }}
                      style={s.bankRow}
                    >
                      <Text
                        style={[c.body, bank === draft.bankName && { fontFamily: font.medium }]}
                      >
                        {bank}
                      </Text>
                    </Pressable>
                  ))}
                </Animated.View>
              ) : null}
              <Field
                placeholder="Account name"
                autoCapitalize="words"
                value={draft.accountName}
                onChangeText={(accountName) => update({ accountName })}
              />
              <Field
                placeholder="Account number"
                keyboardType="number-pad"
                value={draft.accountNumber}
                onChangeText={(t) => update({ accountNumber: t.replace(/\D/g, "").slice(0, 20) })}
              />
            </Animated.View>
          ) : null}
        </Card>
      </Animated.View>

      <Animated.View layout={layout}>
        <Card>
          <ToggleRow
            label="Pay at property"
            hint="Cash or card on arrival"
            value={draft.payAtProperty}
            onChange={(payAtProperty) => update({ payAtProperty })}
          />
        </Card>
      </Animated.View>

      <Animated.View layout={layout} style={{ gap: 8, paddingTop: 4 }}>
        <SectionLabel>Deposit at booking</SectionLabel>
        <View style={s.pills}>
          {DEPOSITS.map((d) => (
            <Pill
              key={d}
              label={d === 0 ? "None" : d === 100 ? "Full amount" : `${d}%`}
              selected={draft.depositPercent === d}
              onPress={() => update({ depositPercent: d })}
            />
          ))}
        </View>
        <Text style={c.muted}>
          {draft.depositPercent > 0 && draft.depositPercent < 100
            ? "The rest is due 14 days before arrival."
            : "Card payments can be added later from Payment methods."}
        </Text>
      </Animated.View>
    </View>
  );
}

export function PolicyStep() {
  const { draft, update } = useOnboarding();
  return (
    <View style={{ gap: 10 }}>
      <OptionRow
        title="Flexible"
        description="Full refund up to 7 days before arrival"
        selected={draft.policy === "flexible"}
        onPress={() => update({ policy: "flexible" })}
      />
      <OptionRow
        title="Moderate"
        description="Full refund up to 14 days before arrival, then 50%"
        badge={<Chip label="Recommended" tone="spark" />}
        selected={draft.policy === "moderate"}
        onPress={() => update({ policy: "moderate" })}
      />
      <OptionRow
        title="Strict"
        description="50% refund up to 30 days before arrival, then none"
        selected={draft.policy === "strict"}
        onPress={() => update({ policy: "strict" })}
      />

      <View style={{ paddingTop: 8, gap: 8 }}>
        <SectionLabel>House rules</SectionLabel>
        <Card>
          <ToggleRow
            label="No smoking indoors"
            value={draft.noSmoking}
            onChange={(noSmoking) => update({ noSmoking })}
          />
          <Divider />
          <ToggleRow
            label="No parties or events"
            value={draft.noParties}
            onChange={(noParties) => update({ noParties })}
          />
          <Divider />
          <ToggleRow
            label="Pets allowed"
            value={draft.petsAllowed}
            onChange={(petsAllowed) => update({ petsAllowed })}
          />
        </Card>
      </View>
    </View>
  );
}

const s = StyleSheet.create({
  price: {
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    borderRadius: 28,
    padding: 20,
    gap: 6,
  },
  priceTop: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  amountRow: { flexDirection: "row", alignItems: "center", gap: 4 },
  symbol: { fontFamily: font.semibold, fontSize: 40, color: colors.obsidian },
  amount: {
    flex: 1,
    fontFamily: font.semibold,
    minWidth: 0,
    fontSize: 44,
    color: colors.obsidian,
    paddingVertical: 4,
    outlineWidth: 0,
  },
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  bankList: {
    borderWidth: 1,
    borderColor: colors.cloud,
    borderRadius: 18,
    paddingHorizontal: 14,
    paddingVertical: 4,
    backgroundColor: colors.subtle,
  },
  bankRow: { paddingVertical: 11 },
});
