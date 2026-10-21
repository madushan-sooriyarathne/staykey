import type { Subscription } from "@staykey/api-client";
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
import { useSubscription } from "@/api/subscription";
import { useTeam } from "@/api/team";
import { Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { Glow } from "@/components/glow";
import { Appear, Card, KV, List, ListRow, Page, ui } from "@/components/kit";
import { LegalLinks } from "@/components/legal-links";
import { font } from "@/components/ui";
import { formatDay, toISO } from "@/data/dates";
import { useProperties } from "@/data/hooks";
import { plural } from "@/data/labels";
import { PLANS, priceLabel, STORE_NAME } from "@/lib/purchases";

const DAY = 24 * 60 * 60 * 1000;
const FREE_DAYS = 60;

const day = (iso: string) => formatDay(toISO(new Date(iso)));

/** The headline and the line under it, from the server's view of the plan. */
function describe(sub: Subscription | undefined): { title: string; line: string } {
  const plan = PLANS.find((p) => p.id === sub?.plan);
  switch (sub?.status) {
    case "active":
      if (sub.source === "web")
        return { title: plan?.name ?? "Your plan", line: `Paid until ${day(sub.until)}` };
      return {
        title: plan?.name ?? "Your plan",
        line:
          plan && sub.period
            ? `${priceLabel(plan, sub.period)}, ${plan.unitsLabel.toLowerCase()}`
            : "",
      };
    case "grace":
      return {
        title: "Choose a plan",
        line: `Your booking page stays open until ${day(sub.until)}`,
      };
    case "lapsed":
      return {
        title: "Booking page paused",
        line: "Guests are asked to message you on WhatsApp until you choose a plan",
      };
    case "trial":
      return {
        title: "Free until your first direct booking",
        line: `${plural(Math.max(0, Math.ceil((Date.parse(sub.until) - Date.now()) / DAY)), "day")} left at most, until ${day(sub.until)}`,
      };
    default:
      return { title: "Your plan", line: "" };
  }
}

/** Plan or free-period status and usage against the plan, as the server sees them. */
export default function SubscriptionScreen() {
  const sub = useSubscription().data;
  const properties = useProperties();
  const team = useTeam().members;
  const plan = PLANS.find((p) => p.id === sub?.plan);
  const fit = PLANS.find((p) => p.id === sub?.fits);
  const store = sub?.status === "active" && sub.source !== "web";
  const web = sub?.status === "active" && sub.source === "web";
  const { title, line } = describe(sub);
  const used =
    sub?.status === "trial"
      ? Math.min(1, Math.max(0, 1 - (Date.parse(sub.until) - Date.now()) / (FREE_DAYS * DAY)))
      : 1;
  const progress = useSharedValue(0);

  useEffect(() => {
    progress.value = withDelay(
      250,
      withTiming(used, { duration: 700, easing: Easing.out(Easing.cubic) }),
    );
  }, [used, progress]);
  const bar = useAnimatedStyle(() => ({ width: `${Math.max(0.03, progress.value) * 100}%` }));

  return (
    <Page title="Subscription">
      <Appear index={0} style={s.dark}>
        <Glow size={260} style={{ right: -110, top: -120 }} intensity={0.8} />
        <Text style={s.darkFaint}>Your plan</Text>
        <Text style={s.darkTitle}>{title}</Text>
        {line ? <Text style={s.darkFaint}>{line}</Text> : null}
        <View style={s.track}>
          <Animated.View style={[s.fill, bar]} />
        </View>
        {web ? null : (
          <Button
            variant="light"
            compact
            title={store ? "Change plan" : "Choose a plan"}
            onPress={() => router.push("/paywall")}
            testID="subscription-choose"
          />
        )}
      </Appear>

      <Appear index={1}>
        <Card title="Usage" meta={fit ? <Tag tone="soft" label={`Fits ${fit.name}`} /> : undefined}>
          <KV label="Properties" value={String(properties.length)} />
          <KV
            label="Bookable units"
            value={`${sub?.units ?? 0}${plan && sub?.status === "active" ? ` of ${plan.units}` : ""}`}
          />
          <KV label="Team members" value={String(team.length)} />
        </Card>
      </Appear>

      <Appear index={2}>
        <List>
          <ListRow
            title="Payment method"
            subtitle={
              store ? `Billed through ${STORE_NAME}` : web ? "Paid on the web" : "Not added"
            }
            onPress={
              store
                ? () =>
                    Linking.openURL(
                      Platform.OS === "android"
                        ? "https://play.google.com/store/account/subscriptions"
                        : "https://apps.apple.com/account/subscriptions",
                    )
                : undefined
            }
          />
        </List>
      </Appear>
      <Text style={[ui.faint, { textAlign: "center", paddingHorizontal: 12 }]}>
        If a plan lapses, your page asks guests to message you on WhatsApp and your data stays safe.
      </Text>
      <LegalLinks />
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
