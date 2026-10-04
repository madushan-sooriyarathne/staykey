import { colors, radius } from "@staykey/tokens";
import * as Clipboard from "expo-clipboard";
import { SymbolView } from "expo-symbols";
import { useEffect, useRef, useState } from "react";
import { Linking, Pressable, ScrollView, Share, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeIn,
  FadeInDown,
  FadeOut,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withDelay,
  withSpring,
  withTiming,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { IconBox, Tag } from "@/components/brand";
import { Button, PressScale } from "@/components/controls";
import { Glow } from "@/components/glow";
import { font } from "@/components/ui";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

/** The one celebration in onboarding. Sharing comes first; alerts are asked for before the OS prompt. */
export default function Live() {
  const { propertyName, bookingPageUrl, completeOnboarding } = useSession();
  const clearDraft = useOnboarding((s) => s.clear);
  const display = bookingPageUrl.replace(/^https?:\/\//, "");
  const shareText = `Book ${propertyName} directly with us: ${bookingPageUrl}`;

  const reduceMotion = useReducedMotion();
  const cardScale = useSharedValue(reduceMotion ? 1 : 0.94);
  const cardOpacity = useSharedValue(0);

  useEffect(() => {
    cardOpacity.value = withTiming(1, { duration: 280, easing: Easing.out(Easing.quad) });
    cardScale.value = withSpring(1, { damping: 16, stiffness: 180, mass: 0.9 });
    const t = setTimeout(() => haptics.success(), 220);
    return () => clearTimeout(t);
  }, [cardOpacity, cardScale]);

  const cardIn = useAnimatedStyle(() => ({
    opacity: cardOpacity.value,
    transform: [{ scale: cardScale.value }],
  }));

  function finish() {
    haptics.step();
    completeOnboarding();
    clearDraft();
  }

  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <View style={s.topBar}>
        <Pressable testID="live-done" hitSlop={10} onPress={finish}>
          <Text style={s.done}>Done</Text>
        </Pressable>
      </View>

      <ScrollView contentContainerStyle={s.content} showsVerticalScrollIndicator={false}>
        <Animated.View style={[s.card, cardIn]}>
          <Glow size={320} style={{ right: -110, top: -120 }} />
          <Confetti />
          <Animated.View entering={FadeIn.delay(260).duration(300)}>
            <Tag tone="spark" label="Live" pulse />
          </Animated.View>
          <Animated.Text
            entering={FadeInDown.delay(320).duration(380).easing(Easing.out(Easing.cubic))}
            style={s.big}
          >
            You're live
          </Animated.Text>
          <Animated.Text
            entering={FadeInDown.delay(380).duration(380).easing(Easing.out(Easing.cubic))}
            style={s.sub}
          >
            Guests can book {propertyName} right now.
          </Animated.Text>
          <Animated.View
            entering={FadeInDown.delay(440).duration(380).easing(Easing.out(Easing.cubic))}
          >
            <LinkPill url={bookingPageUrl} display={display} />
          </Animated.View>
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(520).duration(360)} style={{ gap: 8 }}>
          <Button
            testID="live-whatsapp"
            title="Share on WhatsApp"
            icon={{ ios: "message", android: "chat", web: "chat" }}
            onPress={() => Linking.openURL(`https://wa.me/?text=${encodeURIComponent(shareText)}`)}
          />
          <View style={s.two}>
            <View style={{ flex: 1 }}>
              <Button
                variant="ghost"
                title="More options"
                icon={{ ios: "square.and.arrow.up", android: "share", web: "share" }}
                onPress={() => {
                  haptics.select();
                  Share.share({ message: shareText, url: bookingPageUrl });
                }}
              />
            </View>
            <View style={{ flex: 1 }}>
              <Button
                variant="ghost"
                title="Preview page"
                icon={{ ios: "eye", android: "visibility", web: "visibility" }}
                onPress={() => {
                  haptics.select();
                  Linking.openURL(bookingPageUrl);
                }}
              />
            </View>
          </View>
        </Animated.View>

        <Animated.View entering={FadeInDown.delay(600).duration(360)}>
          <AlertsCard />
        </Animated.View>
      </ScrollView>

      <Animated.View entering={FadeIn.delay(700).duration(300)} style={s.footer}>
        <Button variant="ghost" testID="live-today" title="Go to Today" onPress={finish} />
      </Animated.View>
    </SafeAreaView>
  );
}

function LinkPill({ url, display }: { url: string; display: string }) {
  const [copied, setCopied] = useState(false);
  const timer = useRef<ReturnType<typeof setTimeout> | null>(null);

  useEffect(
    () => () => {
      if (timer.current) clearTimeout(timer.current);
    },
    [],
  );

  return (
    <PressScale
      testID="live-copy"
      accessibilityRole="button"
      accessibilityLabel={copied ? "Link copied" : "Copy booking page link"}
      scaleTo={0.98}
      onPress={async () => {
        await Clipboard.setStringAsync(url);
        haptics.select();
        setCopied(true);
        if (timer.current) clearTimeout(timer.current);
        timer.current = setTimeout(() => setCopied(false), 1800);
      }}
      style={s.pill}
    >
      <SymbolView
        name={{ ios: "link", android: "link", web: "link" }}
        tintColor={colors.ash}
        size={16}
      />
      <Text style={s.pillText} numberOfLines={1}>
        {display}
      </Text>
      {copied ? (
        <Animated.View
          key="copied"
          entering={FadeIn.duration(160)}
          exiting={FadeOut.duration(120)}
          style={s.copied}
        >
          <SymbolView
            name={{ ios: "checkmark", android: "check", web: "check" }}
            tintColor={colors.magenta}
            size={14}
          />
          <Text style={s.copiedText}>Copied</Text>
        </Animated.View>
      ) : (
        <Animated.View key="copy" entering={FadeIn.duration(160)} exiting={FadeOut.duration(120)}>
          <SymbolView
            name={{ ios: "doc.on.doc", android: "content_copy", web: "content_copy" }}
            tintColor={colors.ash}
            size={16}
          />
        </Animated.View>
      )}
    </PressScale>
  );
}

function AlertsCard() {
  const [state, setState] = useState<"ask" | "on" | "later">("ask");
  if (state === "later") return null;

  return (
    <Animated.View exiting={FadeOut.duration(160)} style={s.alerts}>
      {state === "ask" ? (
        <Animated.View key="ask" exiting={FadeOut.duration(120)} style={{ gap: 14 }}>
          <View style={s.alertsRow}>
            <IconBox
              tone="ember"
              name={{ ios: "bell", android: "notifications", web: "notifications" }}
            />
            <View style={{ flex: 1 }}>
              <Text style={s.alertsTitle}>Know the moment a guest books</Text>
              <Text style={s.alertsSub}>Get alerts for new bookings and requests.</Text>
            </View>
          </View>
          <View style={s.two}>
            <View style={{ flex: 1 }}>
              <Button
                variant="ghost"
                title="Not now"
                onPress={() => {
                  haptics.select();
                  setState("later");
                }}
              />
            </View>
            <View style={{ flex: 1 }}>
              <Button
                title="Turn on alerts"
                onPress={() => {
                  // TODO: expo-notifications requestPermissionsAsync, then register the push token.
                  haptics.success();
                  setState("on");
                }}
              />
            </View>
          </View>
        </Animated.View>
      ) : (
        <Animated.View key="on" entering={FadeIn.duration(220)} style={s.alertsRow}>
          <IconBox
            tone="spark"
            name={{
              ios: "bell.badge",
              android: "notifications_active",
              web: "notifications_active",
            }}
          />
          <View style={{ flex: 1 }}>
            <Text style={s.alertsTitle}>Alerts are on</Text>
            <Text style={s.alertsSub}>We'll tell you as soon as a guest books.</Text>
          </View>
        </Animated.View>
      )}
    </Animated.View>
  );
}

/** A small, one-off burst. Pieces fly out with a spring, then fall a little and fade back. */
const PIECES = [
  { x: -96, y: -54, r: 24, w: 6, h: 14, c: colors.ember },
  { x: -40, y: -86, r: -30, w: 9, h: 9, c: colors.magenta, round: true },
  { x: 18, y: -70, r: 50, w: 5, h: 12, c: colors.snow },
  { x: 70, y: -40, r: -18, w: 6, h: 14, c: colors.magenta },
  { x: 104, y: -2, r: 70, w: 8, h: 8, c: colors.ember, round: true },
  { x: -126, y: 6, r: -60, w: 5, h: 12, c: colors.snow },
  { x: 40, y: 36, r: 30, w: 7, h: 7, c: colors.magenta, round: true },
  { x: -60, y: 30, r: 12, w: 5, h: 12, c: colors.ember },
  { x: 128, y: -64, r: -40, w: 5, h: 11, c: colors.snow },
  { x: -10, y: -112, r: 80, w: 6, h: 6, c: colors.ember, round: true },
];

function Confetti() {
  const reduceMotion = useReducedMotion();
  if (reduceMotion) return null;
  return (
    <View pointerEvents="none" style={s.confettiOrigin}>
      {PIECES.map((p, i) => (
        <Piece key={`${p.x}:${p.y}`} piece={p} delay={140 + i * 18} />
      ))}
    </View>
  );
}

function Piece({ piece, delay }: { piece: (typeof PIECES)[number]; delay: number }) {
  const t = useSharedValue(0);
  const fall = useSharedValue(0);

  useEffect(() => {
    t.value = withDelay(delay, withSpring(1, { damping: 14, stiffness: 120, mass: 0.8 }));
    fall.value = withDelay(
      delay + 500,
      withTiming(1, { duration: 1600, easing: Easing.out(Easing.quad) }),
    );
  }, [delay, fall, t]);

  const style = useAnimatedStyle(() => ({
    opacity: Math.min(1, t.value * 2) * (1 - fall.value * 0.45),
    transform: [
      { translateX: piece.x * t.value },
      { translateY: piece.y * t.value + fall.value * 18 },
      { rotate: `${piece.r * t.value + fall.value * piece.r * 0.5}deg` },
      { scale: 0.4 + 0.6 * Math.min(1, t.value) },
    ],
  }));

  return (
    <Animated.View
      style={[
        {
          position: "absolute",
          width: piece.w,
          height: piece.h,
          borderRadius: piece.round ? piece.w : 2,
          backgroundColor: piece.c,
        },
        style,
      ]}
    />
  );
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  topBar: {
    height: 48,
    flexDirection: "row",
    justifyContent: "flex-end",
    alignItems: "center",
    paddingHorizontal: 20,
  },
  done: { fontFamily: font.medium, fontSize: 15, color: colors.iron },
  content: { paddingHorizontal: 16, paddingBottom: 16, gap: 14 },
  card: {
    minHeight: 250,
    padding: 22,
    paddingTop: 28,
    justifyContent: "flex-end",
    borderRadius: radius.cardLarge,
    backgroundColor: colors.graphite,
    overflow: "hidden",
    gap: 6,
  },
  confettiOrigin: { position: "absolute", right: 120, top: 92, width: 0, height: 0 },
  big: {
    fontFamily: font.semibold,
    fontSize: 36,
    lineHeight: 40,
    color: colors.snow,
    marginTop: 6,
  },
  sub: { fontFamily: font.regular, fontSize: 14, color: colors.ash },
  pill: {
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    marginTop: 12,
    height: 46,
    paddingLeft: 14,
    paddingRight: 14,
    borderRadius: radius.button,
    backgroundColor: "rgba(255,255,255,0.08)",
    borderWidth: 1,
    borderColor: "rgba(255,255,255,0.14)",
  },
  pillText: { flex: 1, fontFamily: font.regular, fontSize: 15, color: colors.snow },
  copied: { flexDirection: "row", alignItems: "center", gap: 4 },
  copiedText: { fontFamily: font.medium, fontSize: 13, color: colors.snow },
  two: { flexDirection: "row", gap: 8 },
  alerts: {
    padding: 16,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  alertsRow: { flexDirection: "row", alignItems: "flex-start", gap: 12 },
  alertsTitle: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  alertsSub: {
    fontFamily: font.regular,
    fontSize: 13,
    lineHeight: 18,
    color: colors.fog,
    marginTop: 2,
  },
  footer: { paddingHorizontal: 16, paddingTop: 8, paddingBottom: 8 },
});
