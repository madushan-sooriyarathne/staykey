import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { useEffect } from "react";
import { Linking, Platform, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { Glow } from "@/components/glow";
import { Appear, Card, KV, List, ListRow, Page, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { addDays, formatDay, nightsBetween, today, toISO } from "@/data/dates";
import { useProperties } from "@/data/hooks";
import { plural } from "@/data/labels";
import { physicalUnits } from "@/data/pricing";
import { useData } from "@/data/store";
import { PLANS, planFor, priceLabel, STORE_NAME } from "@/lib/purchases";
import { useSession } from "@/lib/session";

const TRIAL_DAYS = 60;

/** Plan or free-period status, usage against the plan, payment method and billing history. */
export default function Subscription() {
  const { subscription, startedAt } = useSession();
  const properties = useProperties();
  const team = useData((s) => s.team);
  const units = properties.reduce((n, p) => n + physicalUnits(p).length, 0);
  const plan =
    subscription.status === "active" ? PLANS.find((p) => p.id === subscription.plan) : undefined;
  const fit = planFor(units);
  const start = startedAt ? toISO(new Date(startedAt)) : today();
  const ends = addDays(start, TRIAL_DAYS);
  const used = Math.min(TRIAL_DAYS, Math.max(0, nightsBetween(start, today())));
  const progress = useSharedValue(0);

  useEffect(() => {
    progress.value = withDelay(
      250,
      withTiming(plan ? 1 : used / TRIAL_DAYS, { duration: 700, easing: Easing.out(Easing.cubic) }),
    );
  }, [plan, used, progress]);
  const bar = useAnimatedStyle(() => ({ width: `${Math.max(0.03, progress.value) * 100}%` }));

  return (
    <Page title="Subscription">
      <Appear index={0} style={s.dark}>
        <Glow size={260} style={{ right: -110, top: -120 }} intensity={0.8} />
        <Text style={s.darkFaint}>Your plan</Text>
        <Text style={s.darkTitle}>{plan ? plan.name : "Free until your first direct booking"}</Text>
        <Text style={s.darkFaint}>
          {plan && subscription.status === "active"
            ? `${priceLabel(plan, subscription.period)}, ${plan.unitsLabel.toLowerCase()}`
            : `${plural(TRIAL_DAYS - used, "day")} left at most, until ${formatDay(ends)}`}
        </Text>
        <View style={s.track}>
          <Animated.View style={[s.fill, bar]} />
        </View>
        <Button
          variant="light"
          compact
          title={plan ? "Change plan" : "Choose a plan"}
          onPress={() => router.push("/paywall")}
          testID="subscription-choose"
        />
      </Appear>

      <Appear index={1}>
        <Card title="Usage" meta={<Tag tone="soft" label={`Fits ${fit.name}`} />}>
          <KV label="Properties" value={String(properties.length)} />
          <KV label="Bookable units" value={`${units}${plan ? ` of ${plan.units}` : ""}`} />
          <KV label="Team members" value={String(team.length)} />
        </Card>
      </Appear>

      <Appear index={2}>
        <List>
          <ListRow
            title="Payment method"
            subtitle={plan ? `Billed through ${STORE_NAME}` : "Not added"}
            onPress={
              plan
                ? () =>
                    Linking.openURL(
                      Platform.OS === "android"
                        ? "https://play.google.com/store/account/subscriptions"
                        : "https://apps.apple.com/account/subscriptions",
                    )
                : undefined
            }
          />
          <ListRow
            title="Billing history"
            subtitle={
              plan && subscription.status === "active"
                ? `Paid ${priceLabel(plan, subscription.period).split("/")[0]} on ${formatDay(today())}`
                : "No invoices yet"
            }
          />
        </List>
      </Appear>
      <Text style={[ui.faint, { textAlign: "center", paddingHorizontal: 12 }]}>
        If a plan lapses, your page asks guests to message you on WhatsApp and your data stays safe.
      </Text>
    </Page>
  );
}

const s = StyleSheet.create({
  dark: {
    backgroundColor: colors.graphite,
    borderRadius: radius.card,
    padding: 18,
    gap: 6,
    overflow: "hidden",
  },
  darkFaint: { fontFamily: font.regular, fontSize: 13, color: colors.ash },
  darkTitle: { fontFamily: font.semibold, fontSize: 22, lineHeight: 28, color: colors.snow },
  track: {
    height: 4,
    borderRadius: 2,
    backgroundColor: "rgba(255,255,255,0.14)",
    overflow: "hidden",
    marginVertical: 12,
  },
  fill: { height: 4, borderRadius: 2, backgroundColor: colors.magenta },
});
