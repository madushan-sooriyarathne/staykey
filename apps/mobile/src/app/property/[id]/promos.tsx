import { colors, radius } from "@staykey/tokens";
import { useLocalSearchParams } from "expo-router";
import { useEffect, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  LinearTransition,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Field, Stepper } from "@/components/controls";
import { Appear, Hint, money, Segmented, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { formatDay, today } from "@/data/dates";
import { uid } from "@/data/defaults";
import { plural } from "@/data/labels";
import type { Promo } from "@/data/types";
import {
  DashedButton,
  InlineEditor,
  MoneyField,
  SettingsPage,
  useSettings,
} from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

const KEYS = ["promos"] as const;

function statusOf(p: Promo): { label: string; tone: "spark" | "neutral" | "soft" } {
  const day = today();
  if (p.to && p.to < day) return { label: "Ended", tone: "soft" };
  if (p.limit && p.used >= p.limit) return { label: "Used up", tone: "soft" };
  if (p.from && p.from > day) return { label: "Scheduled", tone: "neutral" };
  return { label: "Active", tone: "spark" };
}

/** Discount codes with a value, valid dates and usage limits, and how often each was used. */
export default function PromoCodes() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [editing, setEditing] = useState<Promo | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Promo codes" missing>
        {null}
      </SettingsPage>
    );
  const cur = property.currency;

  const describe = (p: Promo) => {
    const value = p.kind === "percent" ? `${p.amount}% off` : `${money(p.amount, cur)} off`;
    const when =
      p.from && p.to
        ? `, ${formatDay(p.from)} to ${formatDay(p.to)}`
        : p.to
          ? `, until ${formatDay(p.to)}`
          : ", any dates";
    const min = p.minNights ? ` for ${plural(p.minNights, "night")} or more` : "";
    return `${value}${min}${when}${p.note ? `. ${p.note}` : ""}`;
  };

  return (
    <SettingsPage title="Promo codes" dirty={dirty} onSave={save} status={status}>
      {draft.promos.length === 0 ? (
        <Hint>Codes guests enter at checkout, like RETURN10 for returning guests.</Hint>
      ) : null}
      {draft.promos.map((p, i) =>
        editing?.id === p.id ? null : (
          <Appear key={p.id} index={i}>
            <Animated.View layout={LinearTransition.duration(200)}>
              <Pressable
                onPress={() => {
                  haptics.select();
                  setEditing(p);
                }}
                style={({ pressed }) => [s.card, pressed && { opacity: 0.7 }]}
              >
                <View style={s.head}>
                  <Text style={s.code}>{p.code}</Text>
                  <Tag tone={statusOf(p).tone} label={statusOf(p).label} />
                </View>
                <Text style={ui.faint}>{describe(p)}</Text>
                {p.limit ? (
                  <Usage used={p.used} limit={p.limit} />
                ) : (
                  <Text style={ui.faint}>Used {plural(p.used, "time")}</Text>
                )}
              </Pressable>
            </Animated.View>
          </Appear>
        ),
      )}
      {editing ? (
        <PromoEditor
          promo={editing}
          currency={cur}
          isNew={!draft.promos.some((x) => x.id === editing.id)}
          taken={draft.promos.filter((x) => x.id !== editing.id).map((x) => x.code)}
          onCancel={() => setEditing(null)}
          onSave={(p) => {
            set({
              promos: draft.promos.some((x) => x.id === p.id)
                ? draft.promos.map((x) => (x.id === p.id ? p : x))
                : [...draft.promos, p],
            });
            setEditing(null);
          }}
          onDelete={() => {
            set({ promos: draft.promos.filter((x) => x.id !== editing.id) });
            setEditing(null);
          }}
        />
      ) : (
        <DashedButton
          testID="promo-add"
          title="New promo code"
          onPress={() =>
            setEditing({ id: uid("pro"), code: "", kind: "percent", amount: 10, used: 0 })
          }
        />
      )}
    </SettingsPage>
  );
}

function Usage({ used, limit }: { used: number; limit: number }) {
  const w = useSharedValue(0);
  useEffect(() => {
    w.value = withDelay(
      200,
      withTiming(Math.min(1, used / limit), { duration: 600, easing: Easing.out(Easing.cubic) }),
    );
  }, [used, limit, w]);
  const bar = useAnimatedStyle(() => ({ width: `${w.value * 100}%` }));
  return (
    <View style={{ gap: 6, paddingTop: 6 }}>
      <View style={s.track}>
        <Animated.View style={[s.fill, bar]} />
      </View>
      <Text style={ui.faint}>
        Used {used} of {limit} times
      </Text>
    </View>
  );
}

function PromoEditor({
  promo,
  currency,
  isNew,
  taken,
  onCancel,
  onSave,
  onDelete,
}: {
  promo: Promo;
  currency: "USD" | "LKR";
  isNew: boolean;
  taken: string[];
  onCancel: () => void;
  onSave: (p: Promo) => void;
  onDelete: () => void;
}) {
  const [p, setP] = useState(promo);
  const clash = taken.includes(p.code);
  return (
    <InlineEditor
      title={isNew ? "New promo code" : `Edit ${promo.code}`}
      onCancel={onCancel}
      onSave={() =>
        onSave({ ...p, limit: p.limit || undefined, minNights: p.minNights || undefined })
      }
      onDelete={isNew ? undefined : onDelete}
      saveDisabled={p.code.length < 3 || p.amount <= 0 || clash}
    >
      <Field
        testID="promo-code"
        label="Code"
        value={p.code}
        onChangeText={(t) =>
          setP({
            ...p,
            code: t
              .toUpperCase()
              .replace(/[^A-Z0-9]/g, "")
              .slice(0, 16),
          })
        }
        autoCapitalize="characters"
        placeholder="RETURN10"
        error={clash ? "You already have a code with this name." : null}
      />
      <Segmented
        options={[
          { id: "percent", label: "Percent off" },
          { id: "fixed", label: "Amount off" },
        ]}
        value={p.kind}
        onChange={(kind) => setP({ ...p, kind, amount: kind === "percent" ? 10 : 0 })}
      />
      {p.kind === "percent" ? (
        <Stepper
          label="Percent off"
          value={p.amount}
          min={1}
          max={90}
          onChange={(amount) => setP({ ...p, amount })}
        />
      ) : (
        <MoneyField
          label="Amount off"
          value={p.amount}
          currency={currency}
          onChange={(amount) => setP({ ...p, amount })}
        />
      )}
      <Stepper
        label="Usage limit"
        hint="0 means no limit"
        value={p.limit ?? 0}
        min={0}
        max={500}
        onChange={(limit) => setP({ ...p, limit })}
      />
      <Stepper
        label="Minimum nights"
        hint="0 means any stay"
        value={p.minNights ?? 0}
        min={0}
        max={30}
        onChange={(minNights) => setP({ ...p, minNights })}
      />
      <Field
        label="Note for you (optional)"
        value={p.note ?? ""}
        onChangeText={(note) => setP({ ...p, note: note || undefined })}
        placeholder="For returning guests"
      />
    </InlineEditor>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 4,
  },
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 8 },
  code: { fontFamily: font.semibold, fontSize: 16, letterSpacing: 0.5, color: colors.obsidian },
  track: { height: 4, borderRadius: 2, backgroundColor: colors.cloud, overflow: "hidden" },
  fill: { height: 4, borderRadius: 2, backgroundColor: colors.magenta },
});
