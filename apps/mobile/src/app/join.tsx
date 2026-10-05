import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeIn, FadeInDown, ZoomIn } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { IconBox, Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { font } from "@/components/ui";
import { useJoining } from "@/features/auth/enter";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";
import type { Role } from "@/lib/session";

/**
 * Where invited managers and caretakers land. Accepting confirms their number with the same code
 * steps as owners and opens the account they belong to; property setup is skipped entirely.
 * TODO(phase 4): open from the invite link and load the invite from the API instead of this
 * sample, and accept it on the server.
 */
const INVITE: { property: string; invitedBy: string; role: Role; can: string[] } = {
  property: "Kingfisher Villa",
  invitedBy: "Nadeesha Perera",
  role: "caretaker",
  can: [
    "See arrivals and departures",
    "Read guest notes and requests",
    "Contact guests on WhatsApp",
    "Check guests in and out",
  ],
};

const ease = Easing.out(Easing.cubic);

export default function Join() {
  const [accepting, setAccepting] = useState(false);
  const initials = INVITE.property
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2);

  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <ScrollView contentContainerStyle={s.content} showsVerticalScrollIndicator={false}>
        <Animated.View
          entering={ZoomIn.delay(60).springify().damping(18).stiffness(200)}
          style={s.avatar}
        >
          <Text style={s.avatarText}>{initials}</Text>
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(140).duration(380).easing(ease)} style={s.head}>
          <Text style={s.title}>Join {INVITE.property}</Text>
          <Text style={s.subtitle}>
            {INVITE.invitedBy} invited you as a {INVITE.role}.
          </Text>
          <Tag
            label={INVITE.role === "caretaker" ? "Caretaker" : "Manager"}
            style={{ alignSelf: "center", marginTop: 4 }}
          />
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(220).duration(380).easing(ease)} style={s.card}>
          <Text style={s.cardTitle}>You'll be able to</Text>
          {INVITE.can.map((item, i) => (
            <View key={item} style={s.row}>
              <Animated.View
                entering={ZoomIn.delay(360 + i * 80)
                  .springify()
                  .damping(15)
                  .stiffness(240)}
                style={s.tick}
              >
                <SymbolView
                  name={{ ios: "checkmark", android: "check", web: "check" }}
                  tintColor={colors.snow}
                  size={12}
                />
              </Animated.View>
              <Text style={s.rowText}>{item}</Text>
            </View>
          ))}
        </Animated.View>

        <Animated.View entering={FadeIn.delay(560).duration(320)} style={s.lock}>
          <IconBox name={{ ios: "lock", android: "lock", web: "lock" }} />
          <Text style={s.lockText}>Prices, payments and settings stay private to the owner.</Text>
        </Animated.View>
      </ScrollView>

      <Animated.View entering={FadeIn.delay(420).duration(300)} style={s.bar}>
        <View style={{ flex: 1 }}>
          <Button
            variant="ghost"
            title="Decline"
            onPress={() => {
              haptics.back();
              if (router.canGoBack()) router.back();
              else router.replace("/welcome");
            }}
          />
        </View>
        <View style={{ flex: 1 }}>
          <Button
            testID="join-accept"
            title="Accept invite"
            loading={accepting}
            onPress={() => {
              setAccepting(true);
              haptics.step();
              useJoining.setState({ joining: true });
              useOnboarding.getState().setStep("phone");
              router.push("/onboarding");
              setAccepting(false);
            }}
          />
        </View>
      </Animated.View>
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  content: { paddingHorizontal: 16, paddingTop: 40, paddingBottom: 20, gap: 16 },
  avatar: {
    alignSelf: "center",
    width: 72,
    height: 72,
    borderRadius: 36,
    backgroundColor: colors.graphite,
    alignItems: "center",
    justifyContent: "center",
  },
  avatarText: { fontFamily: font.semibold, fontSize: 24, color: colors.snow },
  head: { alignItems: "center", gap: 6, paddingHorizontal: 12, paddingBottom: 8 },
  title: {
    fontFamily: font.semibold,
    fontSize: 28,
    lineHeight: 34,
    color: colors.obsidian,
    textAlign: "center",
  },
  subtitle: {
    fontFamily: font.regular,
    fontSize: 15,
    lineHeight: 21,
    color: colors.steel,
    textAlign: "center",
  },
  card: {
    padding: 18,
    paddingBottom: 8,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  cardTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian, marginBottom: 4 },
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10 },
  tick: {
    width: 22,
    height: 22,
    borderRadius: 11,
    backgroundColor: colors.ember,
    alignItems: "center",
    justifyContent: "center",
  },
  rowText: { flex: 1, fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  lock: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 14,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  lockText: {
    flex: 1,
    fontFamily: font.regular,
    fontSize: 14,
    lineHeight: 20,
    color: colors.steel,
  },
  bar: { flexDirection: "row", gap: 8, paddingHorizontal: 16, paddingTop: 8, paddingBottom: 8 },
});
