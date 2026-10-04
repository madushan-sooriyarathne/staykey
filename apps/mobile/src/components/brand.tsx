import { colors, radius } from "@staykey/tokens";
import { SymbolView, type SymbolViewProps } from "expo-symbols";
import { type ReactNode, useEffect } from "react";
import { type StyleProp, StyleSheet, Text, View, type ViewStyle } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import { font } from "@/components/ui";

type Tone = "neutral" | "soft" | "ember" | "spark" | "onDark" | "ink" | "dash";

const TONES: Record<Tone, { bg: string; fg: string; border: string }> = {
  neutral: { bg: colors.snow, fg: colors.iron, border: colors.cloud },
  soft: { bg: colors.paper, fg: colors.steel, border: colors.paper },
  ink: { bg: colors.obsidian, fg: colors.snow, border: colors.obsidian },
  dash: { bg: "transparent", fg: colors.iron, border: colors.ash },
  ember: { bg: colors.emberTint, fg: colors.emberInk, border: colors.emberTint },
  spark: { bg: colors.magentaTint, fg: colors.magentaInk, border: colors.magentaTint },
  onDark: { bg: "rgba(255,255,255,0.1)", fg: colors.snow, border: "rgba(255,255,255,0.16)" },
};

/** Small rounded label. Spark marks wins and live status, Ember marks things to act on. */
export function Tag({
  label,
  tone = "neutral",
  pulse,
  icon,
  style,
}: {
  label: string;
  tone?: Tone;
  pulse?: boolean;
  icon?: SymbolViewProps["name"];
  style?: StyleProp<ViewStyle>;
}) {
  const t = TONES[tone];
  return (
    <View
      style={[
        s.tag,
        { backgroundColor: t.bg, borderColor: t.border },
        tone === "dash" && { borderStyle: "dashed" },
        style,
      ]}
    >
      {pulse ? <PulseDot /> : null}
      {icon ? <SymbolView name={icon} tintColor={t.fg} size={13} /> : null}
      <Text style={[s.tagText, { color: t.fg }]}>{label}</Text>
    </View>
  );
}

/** The live indicator: a Magenta Spark dot with a soft ring that breathes outwards. */
export function PulseDot({ size = 6 }: { size?: number }) {
  const reduceMotion = useReducedMotion();
  const t = useSharedValue(0);

  useEffect(() => {
    if (reduceMotion) return;
    t.value = withRepeat(
      withTiming(1, { duration: 1600, easing: Easing.out(Easing.quad) }),
      -1,
      false,
    );
  }, [reduceMotion, t]);

  const ring = useAnimatedStyle(() => ({
    opacity: 0.45 * (1 - t.value),
    transform: [{ scale: 1 + t.value * 1.6 }],
  }));

  return (
    <View style={{ width: size, height: size }}>
      <Animated.View
        style={[
          {
            position: "absolute",
            width: size,
            height: size,
            borderRadius: size,
            backgroundColor: colors.magenta,
          },
          ring,
        ]}
      />
      <View
        style={{ width: size, height: size, borderRadius: size, backgroundColor: colors.magenta }}
      />
    </View>
  );
}

/** Rounded square that holds an icon in lists and cards. */
export function IconBox({
  name,
  tone = "neutral",
  size = 40,
}: {
  name: SymbolViewProps["name"];
  tone?: "neutral" | "surface" | "ember" | "spark" | "onDark";
  size?: number;
}) {
  const palette = {
    neutral: { bg: colors.paper, fg: colors.iron },
    surface: { bg: colors.snow, fg: colors.iron },
    ember: { bg: colors.emberTint, fg: colors.ember },
    spark: { bg: colors.magentaTint, fg: colors.magentaInk },
    onDark: { bg: "rgba(255,255,255,0.1)", fg: colors.snow },
  }[tone];
  return (
    <View
      style={[
        s.iconBox,
        { width: size, height: size, backgroundColor: palette.bg },
        tone === "surface" && { borderWidth: 1, borderColor: colors.cloud },
      ]}
    >
      <SymbolView name={name} tintColor={palette.fg} size={Math.round(size * 0.48)} />
    </View>
  );
}

/** Round tick used in checklists. */
export function Tick({ done, tone = "obsidian" }: { done: boolean; tone?: "obsidian" | "ember" }) {
  const fill = tone === "ember" ? colors.ember : colors.obsidian;
  return (
    <View style={[s.tick, done && { backgroundColor: fill, borderColor: fill }]}>
      {done ? (
        <SymbolView
          name={{ ios: "checkmark", android: "check", web: "check" }}
          tintColor={colors.snow}
          size={12}
        />
      ) : null}
    </View>
  );
}

/** A tappable-looking list row for cards: icon or tick, text, trailing element. */
export function Row({
  leading,
  title,
  subtitle,
  trailing,
  muted,
}: {
  leading?: ReactNode;
  title: string;
  subtitle?: string;
  trailing?: ReactNode;
  muted?: boolean;
}) {
  return (
    <View style={s.row}>
      {leading}
      <View style={{ flex: 1, gap: 2 }}>
        <Text style={[s.rowTitle, muted && { color: colors.fog }]}>{title}</Text>
        {subtitle ? <Text style={s.rowSub}>{subtitle}</Text> : null}
      </View>
      {trailing}
    </View>
  );
}

export function Chevron() {
  return (
    <SymbolView
      name={{ ios: "chevron.right", android: "chevron_right", web: "chevron_right" }}
      tintColor={colors.ash}
      size={14}
    />
  );
}

const s = StyleSheet.create({
  tag: {
    flexDirection: "row",
    alignItems: "center",
    alignSelf: "flex-start",
    gap: 6,
    height: 26,
    paddingHorizontal: 10,
    borderRadius: radius.badge,
    borderWidth: 1,
  },
  tagText: { fontFamily: font.medium, fontSize: 12.5 },
  iconBox: { borderRadius: 14, alignItems: "center", justifyContent: "center" },
  tick: {
    width: 22,
    height: 22,
    borderRadius: 11,
    borderWidth: 1.5,
    borderColor: colors.mist,
    alignItems: "center",
    justifyContent: "center",
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12 },
  rowTitle: { fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  rowSub: { fontFamily: font.regular, fontSize: 13, lineHeight: 18, color: colors.fog },
});
