import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import type { SymbolViewProps } from "expo-symbols";
import { useEffect } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeIn,
  FadeInDown,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withDelay,
  withRepeat,
  withTiming,
  ZoomIn,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { IconBox, Tag } from "@/components/brand";
import { Button, TextLink } from "@/components/controls";
import { Glow } from "@/components/glow";
import { font } from "@/components/ui";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";

const BENEFITS: { icon: SymbolViewProps["name"]; label: string }[] = [
  {
    icon: { ios: "globe", android: "public", web: "public" },
    label: "Your own booking page, live today",
  },
  {
    icon: { ios: "calendar", android: "calendar_month", web: "calendar_month" },
    label: "Direct and OTA bookings in one calendar",
  },
  {
    icon: { ios: "percent", android: "percent", web: "percent" },
    label: "No commission on direct bookings",
  },
];

/** Entrance timing, so the hero, headline and actions arrive in one calm sequence. */
const T = { card1: 80, cells: 360, card2: 620, headline: 200, benefits: 360, actions: 560 };

export default function Welcome() {
  const resumeAt = useOnboarding((s) => s.stepId);
  const setStep = useOnboarding((s) => s.setStep);

  function start() {
    haptics.step();
    router.push("/onboarding");
  }

  function signIn() {
    // Sign in uses the same phone and code steps. TODO: when the API knows the number, skip
    // property setup and load the owner's account instead.
    haptics.step();
    setStep("phone");
    router.push("/onboarding");
  }

  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <ScrollView
        contentContainerStyle={s.scroll}
        bounces={false}
        showsVerticalScrollIndicator={false}
      >
        <Hero />

        <View style={s.copy}>
          <Animated.Text
            entering={FadeInDown.delay(T.headline).duration(420).easing(Easing.out(Easing.cubic))}
            style={s.headline}
          >
            Take direct bookings in 10 minutes
          </Animated.Text>
          <Animated.View entering={FadeIn.delay(T.headline + 120).duration(320)}>
            <Tag tone="spark" label="Free until your first direct booking" pulse />
          </Animated.View>
        </View>

        <View style={s.benefits}>
          {BENEFITS.map((b, i) => (
            <Animated.View
              key={b.label}
              entering={FadeInDown.delay(T.benefits + i * 70)
                .duration(360)
                .easing(Easing.out(Easing.cubic))}
              style={s.benefit}
            >
              <IconBox tone="surface" name={b.icon} />
              <Text style={s.benefitText}>{b.label}</Text>
            </Animated.View>
          ))}
        </View>
      </ScrollView>

      <Animated.View entering={FadeIn.delay(T.actions).duration(320)} style={s.actions}>
        <Button
          testID="welcome-start"
          title={resumeAt ? "Continue setup" : "Get started"}
          onPress={start}
        />
        <Button variant="ghost" title="I have an account" onPress={signIn} />
        <View style={{ paddingTop: 8 }}>
          <TextLink
            testID="welcome-join"
            title="I was invited to a team"
            onPress={() => {
              haptics.select();
              router.push("/join");
            }}
          />
        </View>
      </Animated.View>
    </SafeAreaView>
  );
}

/** Two product snippets: a live calendar filling with direct bookings, and a paid booking alert. */
function Hero() {
  return (
    <View style={s.hero}>
      <Glow size={300} style={{ left: 40, top: 0 }} intensity={0.9} />

      <Float delay={0} style={[s.heroCard, { left: 0, right: 28, top: 18 }]}>
        <Animated.View
          entering={FadeInDown.delay(T.card1).springify().damping(20).stiffness(160)}
          style={s.cardInner}
        >
          <View style={s.cardTitle}>
            <Text style={s.cardName}>Kingfisher Villa</Text>
            <Tag tone="spark" label="Live" pulse />
          </View>
          <View style={s.days}>
            {[1, 2, 3, 4, 5, 6, 7].map((d) => {
              const direct = d >= 4;
              return (
                <View key={d} style={s.day}>
                  {direct ? (
                    <Animated.View
                      entering={ZoomIn.delay(T.cells + (d - 4) * 90)
                        .springify()
                        .damping(16)
                        .stiffness(220)}
                      style={[StyleSheet.absoluteFill, s.dayDirect]}
                    />
                  ) : null}
                  <Text style={[s.dayText, direct ? s.dayTextDirect : s.dayTextPast]}>{d}</Text>
                </View>
              );
            })}
          </View>
        </Animated.View>
      </Float>

      <Float delay={900} style={[s.heroCard, { left: 20, right: 0, top: 142 }]}>
        <Animated.View
          entering={FadeInDown.delay(T.card2).springify().damping(20).stiffness(160)}
          style={[s.cardInner, s.alert]}
        >
          <IconBox
            tone="ember"
            name={{ ios: "bell", android: "notifications", web: "notifications" }}
          />
          <View style={{ flex: 1 }}>
            <Text style={s.alertTitle}>New direct booking</Text>
            <Text style={s.alertSub}>Emma Larsen, $832</Text>
          </View>
          <Tag tone="spark" label="Paid" />
        </Animated.View>
      </Float>
    </View>
  );
}

/** Slow vertical drift that gives the hero a little life once it has settled. */
function Float({
  children,
  delay,
  style,
}: {
  children: React.ReactNode;
  delay: number;
  style: object;
}) {
  const reduceMotion = useReducedMotion();
  const y = useSharedValue(0);

  useEffect(() => {
    if (reduceMotion) return;
    y.value = withDelay(
      1200 + delay,
      withRepeat(withTiming(-4, { duration: 2800, easing: Easing.inOut(Easing.sin) }), -1, true),
    );
  }, [delay, reduceMotion, y]);

  const animated = useAnimatedStyle(() => ({ transform: [{ translateY: y.value }] }));
  return <Animated.View style={[style, animated]}>{children}</Animated.View>;
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  scroll: { paddingBottom: 12 },
  hero: { height: 236, marginHorizontal: 16, marginTop: 8 },
  heroCard: { position: "absolute" },
  cardInner: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
  },
  cardTitle: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  cardName: { fontFamily: font.medium, fontSize: 14, color: colors.obsidian },
  days: { flexDirection: "row", gap: 4, marginTop: 12 },
  day: {
    flex: 1,
    height: 38,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  },
  dayDirect: { backgroundColor: colors.ember, borderRadius: 12 },
  dayText: { fontFamily: font.regular, fontSize: 13 },
  dayTextPast: { color: colors.ash },
  dayTextDirect: { color: colors.snow, fontFamily: font.medium },
  alert: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 14 },
  alertTitle: { fontFamily: font.medium, fontSize: 14, color: colors.obsidian },
  alertSub: { fontFamily: font.regular, fontSize: 13, color: colors.fog, marginTop: 1 },
  copy: { paddingHorizontal: 20, paddingTop: 18, gap: 14 },
  headline: {
    fontFamily: font.semibold,
    fontSize: 32,
    lineHeight: 37,
    color: colors.obsidian,
    letterSpacing: -0.4,
  },
  benefits: { paddingHorizontal: 20, paddingTop: 22, gap: 14 },
  benefit: { flexDirection: "row", alignItems: "center", gap: 14 },
  benefitText: { flex: 1, fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  actions: { paddingHorizontal: 16, paddingTop: 8, paddingBottom: 8, gap: 8 },
});
