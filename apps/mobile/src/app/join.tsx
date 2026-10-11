import { colors, radius } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeIn, FadeInDown, ZoomIn } from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { messageFor } from "@/api/errors";
import { acceptInvite, useAfterJoin, useInvitePreview } from "@/api/team";
import { IconBox, Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { font } from "@/components/ui";
import { openAccount, useJoining } from "@/features/auth/enter";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

/** What each role can do, as the invite promises it. */
const CAN: Record<"manager" | "caretaker", string[]> = {
  caretaker: [
    "See arrivals and departures",
    "Read guest notes and requests",
    "Contact guests on WhatsApp",
    "Check guests in and out",
  ],
  manager: [
    "Run bookings and the calendar",
    "Approve requests and record payments",
    "Change rates and property settings",
    "Contact guests on WhatsApp",
  ],
};

/**
 * Where invited managers and caretakers land, from the link in their WhatsApp invite
 * (staykey://join?token=...). Accepting confirms their number with the same code steps as owners;
 * the verified sign-in accepts the invite on the server and opens the account. Property setup is
 * skipped entirely.
 */
const ease = Easing.out(Easing.cubic);

export default function Join() {
  const { token } = useLocalSearchParams<{ token?: string }>();
  const preview = useInvitePreview(token);
  const invite = preview.data;
  const [accepting, setAccepting] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const onboarded = useSession((st) => st.onboarded);
  const afterJoin = useAfterJoin();
  const place = invite
    ? invite.propertyNames.length === 1
      ? invite.propertyNames[0]
      : invite.accountName
    : "";
  const initials = (place || "StayKey")
    .split(" ")
    .map((w) => w[0])
    .join("")
    .slice(0, 2);

  if (!invite) {
    return (
      <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
        <View style={[s.content, { flex: 1, justifyContent: "center" }]}>
          <Text style={s.title}>
            {preview.isPending && token ? "Opening your invite" : "Open your invite link"}
          </Text>
          <Text style={s.subtitle}>
            {!token
              ? "Tap the link the owner sent you on WhatsApp to join their team."
              : preview.isError
                ? messageFor(preview.error)
                : "Checking the invite."}
          </Text>
        </View>
        <View style={s.bar}>
          <View style={{ flex: 1 }}>
            <Button
              variant="ghost"
              title="Back"
              onPress={() => (router.canGoBack() ? router.back() : router.replace("/welcome"))}
            />
          </View>
        </View>
      </SafeAreaView>
    );
  }
  const can = CAN[invite.role];

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
          <Text style={s.title}>Join {place}</Text>
          <Text style={s.subtitle}>
            {invite.invitedByName || "The owner"} invited you as a {invite.role}
            {invite.propertyNames.length > 1 ? ` for ${invite.propertyNames.join(", ")}` : ""}.
          </Text>
          <Tag
            label={invite.role === "caretaker" ? "Caretaker" : "Manager"}
            style={{ alignSelf: "center", marginTop: 4 }}
          />
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(220).duration(380).easing(ease)} style={s.card}>
          <Text style={s.cardTitle}>You'll be able to</Text>
          {can.map((item, i) => (
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

        {error ? <Text style={[s.lockText, { textAlign: "center" }]}>{error}</Text> : null}

        {invite.role === "caretaker" ? (
          <Animated.View entering={FadeIn.delay(560).duration(320)} style={s.lock}>
            <IconBox name={{ ios: "lock", android: "lock", web: "lock" }} />
            <Text style={s.lockText}>Prices, payments and settings stay private to the owner.</Text>
          </Animated.View>
        ) : null}
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
            onPress={async () => {
              setAccepting(true);
              setError(null);
              haptics.step();
              if (!onboarded) {
                // Confirm the number first; the verified sign-in accepts the invite.
                useJoining.setState({ joining: true, token });
                useOnboarding.getState().setStep("phone");
                router.push("/onboarding");
                setAccepting(false);
                return;
              }
              // Already signed in on this phone: join straight away and switch to the account.
              try {
                const account = await acceptInvite(token ?? "");
                afterJoin();
                if (!(await openAccount(account))) throw new Error("offline");
                haptics.success();
                router.replace("/");
              } catch (e) {
                haptics.error();
                setError(messageFor(e));
              } finally {
                setAccepting(false);
              }
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
