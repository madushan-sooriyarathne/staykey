import { formatMoney } from "@staykey/api-client";
import { colors, radius } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { SymbolView } from "expo-symbols";
import { useEffect, useState } from "react";
import {
  type LayoutChangeEvent,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import Animated, {
  Easing,
  FadeIn,
  FadeInDown,
  LinearTransition,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { IconBox, Tag } from "@/components/brand";
import { Button, PressScale, Radio, TextLink } from "@/components/controls";
import { font } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { spring } from "@/lib/motion";
import {
  INCLUDED,
  type Period,
  PLANS,
  type Plan,
  type PlanId,
  planFor,
  priceLabel,
  purchase,
  restore,
  STORE_NAME,
} from "@/lib/purchases";
import { useSession } from "@/lib/session";

const ease = Easing.out(Easing.cubic);
/** Typical OTA commission the savings line is based on. */
const OTA_COMMISSION = 0.15;
const FALLBACK_RATE = { USD: 18000, LKR: 5400000 } as const;

/**
 * Shown after the first direct booking (reason=first-booking) and from More, Subscription.
 * Plans differ only by bookable units, so the plan that fits the property is picked for the owner.
 */
export default function Paywall() {
  const { reason } = useLocalSearchParams<{ reason?: string }>();
  const { setup, subscription, subscribe } = useSession();
  const fit = planFor(setup.units);

  const [period, setPeriod] = useState<Period>("yearly");
  const [selected, setSelected] = useState<PlanId>(
    subscription.status === "active" ? subscription.plan : fit.id,
  );
  const [status, setStatus] = useState<"idle" | "buying" | "done">("idle");
  const [restoreNote, setRestoreNote] = useState<string | null>(null);

  const plan = PLANS.find((p) => p.id === selected) ?? fit;
  const rate = setup.nightlyRate > 0 ? setup.nightlyRate : FALLBACK_RATE[setup.currency];
  const saved = Math.round((rate * 3 * OTA_COMMISSION) / 100) * 100;

  async function buy() {
    if (status !== "idle") return;
    setStatus("buying");
    const result = await purchase(plan.id, period);
    if (!result.ok) {
      setStatus("idle");
      if (!result.cancelled) haptics.error();
      return;
    }
    haptics.success();
    subscribe(plan.id, period);
    setStatus("done");
    setTimeout(() => router.back(), 1100);
  }

  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <View style={s.topBar}>
        <Pressable
          accessibilityRole="button"
          accessibilityLabel="Close"
          hitSlop={8}
          onPress={() => {
            haptics.back();
            router.back();
          }}
          style={s.close}
        >
          <SymbolView
            name={{ ios: "xmark", android: "close", web: "close" }}
            tintColor={colors.iron}
            size={16}
          />
        </Pressable>
      </View>

      <ScrollView contentContainerStyle={s.content} showsVerticalScrollIndicator={false}>
        <Animated.View entering={FadeInDown.duration(380).easing(ease)} style={{ gap: 8 }}>
          {reason === "first-booking" ? (
            <Tag tone="spark" label="Your first direct booking is in" pulse />
          ) : null}
          <Text style={s.title}>Choose your plan</Text>
          <Text style={s.subtitle}>
            {reason === "first-booking"
              ? "Pick a plan within 7 days to keep your booking page open. Every plan includes every feature."
              : "StayKey is free until your first direct booking. Every plan includes every feature, so pick the one that fits your rooms."}
          </Text>
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(60).duration(380).easing(ease)} style={s.savings}>
          <IconBox tone="ember" name={{ ios: "percent", android: "percent", web: "percent" }} />
          <Text style={s.savingsText}>
            One 3-night direct booking saves you about{" "}
            <Text style={s.savingsAmount}>{formatMoney(saved, setup.currency)}</Text> in OTA
            commission.
          </Text>
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(120).duration(380).easing(ease)}>
          <PeriodToggle value={period} onChange={setPeriod} />
        </Animated.View>

        <View style={{ gap: 10 }}>
          {PLANS.map((p, i) => (
            <Animated.View
              key={p.id}
              entering={FadeInDown.delay(180 + i * 60)
                .duration(380)
                .easing(ease)}
            >
              <PlanCard
                plan={p}
                period={period}
                selected={selected === p.id}
                recommended={p.id === fit.id}
                tooSmall={p.units < setup.units}
                current={subscription.status === "active" && subscription.plan === p.id}
                onPress={() => setSelected(p.id)}
              />
            </Animated.View>
          ))}
        </View>

        <Animated.View entering={FadeIn.delay(420).duration(320)} style={s.included}>
          <Text style={s.includedTitle}>Included in every plan</Text>
          {INCLUDED.map((item) => (
            <View key={item} style={s.includedRow}>
              <SymbolView
                name={{ ios: "checkmark", android: "check", web: "check" }}
                tintColor={colors.ember}
                size={14}
              />
              <Text style={s.includedText}>{item}</Text>
            </View>
          ))}
        </Animated.View>

        <Text style={s.fine}>
          Billed through {STORE_NAME}, cancel anytime. If a plan lapses, your page asks guests to
          message you on WhatsApp and your data stays safe.
        </Text>
      </ScrollView>

      <View style={s.footer}>
        <Button
          testID="paywall-buy"
          title={
            status === "done"
              ? `You're on ${plan.name}`
              : `Start ${plan.name} for ${priceLabel(plan, period)}`
          }
          icon={
            status === "done" ? { ios: "checkmark", android: "check", web: "check" } : undefined
          }
          loading={status === "buying"}
          disabled={plan.units < setup.units}
          onPress={buy}
        />
        <View style={{ paddingTop: 12 }}>
          {restoreNote ? (
            <Animated.Text entering={FadeIn.duration(200)} style={s.restoreNote}>
              {restoreNote}
            </Animated.Text>
          ) : (
            <TextLink
              title="Restore purchases"
              onPress={async () => {
                haptics.select();
                const r = await restore();
                setRestoreNote(
                  r.restored
                    ? "Your plan is restored."
                    : "No previous purchases found on this account.",
                );
              }}
            />
          )}
        </View>
      </View>
    </SafeAreaView>
  );
}

/** Monthly or yearly. The thumb springs between halves; yearly carries the saving. */
function PeriodToggle({ value, onChange }: { value: Period; onChange: (p: Period) => void }) {
  const [width, setWidth] = useState(0);
  const x = useSharedValue(value === "yearly" ? 1 : 0);

  useEffect(() => {
    x.value = withSpring(value === "yearly" ? 1 : 0, spring.control);
  }, [value, x]);

  const thumb = useAnimatedStyle(() => ({
    width: width / 2 - 4,
    transform: [{ translateX: x.value * (width / 2) }],
  }));

  const options: { id: Period; label: string }[] = [
    { id: "monthly", label: "Monthly" },
    { id: "yearly", label: "Yearly" },
  ];

  return (
    <View
      style={s.toggle}
      onLayout={(e: LayoutChangeEvent) => setWidth(e.nativeEvent.layout.width)}
    >
      {width > 0 ? <Animated.View style={[s.thumb, thumb]} /> : null}
      {options.map((o) => (
        <Pressable
          key={o.id}
          testID={`period-${o.id}`}
          accessibilityRole="tab"
          accessibilityState={{ selected: value === o.id }}
          style={s.toggleOption}
          onPress={() => {
            if (value !== o.id) haptics.select();
            onChange(o.id);
          }}
        >
          <Text style={[s.toggleText, value === o.id && s.toggleTextOn]}>{o.label}</Text>
          {o.id === "yearly" ? (
            <Tag tone="spark" label="2 months free" style={s.toggleTag} />
          ) : null}
        </Pressable>
      ))}
    </View>
  );
}

function PlanCard({
  plan,
  period,
  selected,
  recommended,
  tooSmall,
  current,
  onPress,
}: {
  plan: Plan;
  period: Period;
  selected: boolean;
  recommended: boolean;
  tooSmall: boolean;
  current: boolean;
  onPress: () => void;
}) {
  const price = period === "monthly" ? plan.monthly : plan.yearly;
  const perMonth = (plan.yearly / 12).toFixed(2).replace(/\.00$/, "");
  return (
    <PressScale
      testID={`plan-${plan.id}`}
      accessibilityRole="radio"
      accessibilityState={{ selected, disabled: tooSmall }}
      disabled={tooSmall}
      scaleTo={0.985}
      onPress={() => {
        if (!selected) haptics.select();
        onPress();
      }}
      style={[s.plan, selected && s.planOn, tooSmall && { opacity: 0.45 }]}
    >
      <Radio on={selected} />
      <View style={{ flex: 1, gap: 2 }}>
        <View style={s.planTitleRow}>
          <Text style={s.planName}>{plan.name}</Text>
          {current ? (
            <Tag tone="spark" label="Current" />
          ) : recommended ? (
            <Tag tone="ember" label="Fits your property" />
          ) : null}
        </View>
        <Text style={s.planUnits}>
          {tooSmall ? `${plan.unitsLabel}, too small for yours` : plan.unitsLabel}
        </Text>
      </View>
      <Animated.View
        key={period}
        entering={FadeIn.duration(200)}
        layout={LinearTransition}
        style={{ alignItems: "flex-end" }}
      >
        <Text style={s.planPrice}>
          ${price}
          <Text style={s.planPeriod}>{period === "monthly" ? "/mo" : "/yr"}</Text>
        </Text>
        {period === "yearly" ? <Text style={s.planPerMonth}>${perMonth}/mo</Text> : null}
      </Animated.View>
    </PressScale>
  );
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  topBar: {
    height: 52,
    flexDirection: "row",
    justifyContent: "flex-end",
    alignItems: "center",
    paddingHorizontal: 16,
  },
  close: {
    width: 36,
    height: 36,
    borderRadius: 18,
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    alignItems: "center",
    justifyContent: "center",
  },
  content: { paddingHorizontal: 16, paddingBottom: 20, gap: 16 },
  title: { fontFamily: font.semibold, fontSize: 28, lineHeight: 34, color: colors.obsidian },
  subtitle: { fontFamily: font.regular, fontSize: 15, lineHeight: 21, color: colors.steel },
  savings: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 14,
    borderRadius: radius.card,
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
  },
  savingsText: {
    flex: 1,
    fontFamily: font.regular,
    fontSize: 14,
    lineHeight: 20,
    color: colors.graphite,
  },
  savingsAmount: { fontFamily: font.semibold, color: colors.obsidian },
  toggle: {
    flexDirection: "row",
    height: 48,
    padding: 4,
    borderRadius: radius.button + 2,
    backgroundColor: colors.cloud,
  },
  thumb: {
    position: "absolute",
    top: 4,
    left: 4,
    bottom: 4,
    borderRadius: radius.button - 2,
    backgroundColor: colors.snow,
  },
  toggleOption: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 6,
  },
  toggleText: { fontFamily: font.medium, fontSize: 14, color: colors.steel },
  toggleTextOn: { color: colors.obsidian },
  toggleTag: { height: 22, paddingHorizontal: 8 },
  plan: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 16,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  planOn: { borderColor: colors.obsidian, borderWidth: 1.5 },
  planTitleRow: { flexDirection: "row", alignItems: "center", gap: 8, flexWrap: "wrap" },
  planName: { fontFamily: font.semibold, fontSize: 16, color: colors.obsidian },
  planUnits: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  planPrice: { fontFamily: font.semibold, fontSize: 20, color: colors.obsidian },
  planPeriod: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  planPerMonth: { fontFamily: font.regular, fontSize: 12, color: colors.fog, marginTop: 1 },
  included: {
    padding: 18,
    gap: 10,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  includedTitle: {
    fontFamily: font.semibold,
    fontSize: 15,
    color: colors.obsidian,
    marginBottom: 2,
  },
  includedRow: { flexDirection: "row", alignItems: "center", gap: 10 },
  includedText: { flex: 1, fontFamily: font.regular, fontSize: 14, color: colors.graphite },
  fine: {
    fontFamily: font.regular,
    fontSize: 12,
    lineHeight: 17,
    color: colors.fog,
    textAlign: "center",
    paddingHorizontal: 8,
  },
  footer: { paddingHorizontal: 16, paddingTop: 10, paddingBottom: 8 },
  restoreNote: { fontFamily: font.regular, fontSize: 13, color: colors.fog, textAlign: "center" },
});
