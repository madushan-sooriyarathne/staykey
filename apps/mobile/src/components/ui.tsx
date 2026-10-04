import { colors, fontSize, radius } from "@staykey/tokens";
import type { ReactNode } from "react";
import {
  Pressable,
  type PressableProps,
  ScrollView,
  StyleSheet,
  Text,
  type TextProps,
  View,
  type ViewProps,
} from "react-native";
import { SafeAreaView } from "react-native-safe-area-context";

/** Font family names registered in src/app/_layout.tsx. */
export const font = {
  regular: "DMSans_400Regular",
  medium: "DMSans_500Medium",
  semibold: "DMSans_600SemiBold",
} as const;

export function Screen({
  title,
  eyebrow,
  right,
  children,
}: {
  title: string;
  eyebrow?: string;
  right?: ReactNode;
  children: ReactNode;
}) {
  return (
    <SafeAreaView edges={["top"]} style={styles.screen}>
      <ScrollView contentContainerStyle={styles.content}>
        <View style={styles.header}>
          <View>
            {eyebrow ? <Label tone="faint">{eyebrow}</Label> : null}
            <Text style={styles.title}>{title}</Text>
          </View>
          {right}
        </View>
        {children}
      </ScrollView>
    </SafeAreaView>
  );
}

export function Card({ style, ...props }: ViewProps) {
  return <View style={[styles.card, style]} {...props} />;
}

export function Heading({ style, ...props }: TextProps) {
  return <Text style={[styles.heading, style]} {...props} />;
}

export function Body({ style, ...props }: TextProps) {
  return <Text style={[styles.body, style]} {...props} />;
}

export function Label({
  tone = "muted",
  style,
  ...props
}: TextProps & { tone?: "muted" | "faint" }) {
  return (
    <Text
      style={[styles.label, { color: tone === "muted" ? colors.steel : colors.fog }, style]}
      {...props}
    />
  );
}

export function Button({
  title,
  variant = "primary",
  style,
  ...props
}: PressableProps & { title: string; variant?: "primary" | "ghost" }) {
  const primary = variant === "primary";
  return (
    <Pressable
      accessibilityRole="button"
      style={(state) => [
        styles.button,
        primary ? styles.buttonPrimary : styles.buttonGhost,
        state.pressed && { opacity: 0.85 },
        props.disabled && { opacity: 0.4 },
        typeof style === "function" ? style(state) : style,
      ]}
      {...props}
    >
      <Text style={[styles.buttonText, { color: primary ? colors.snow : colors.iron }]}>
        {title}
      </Text>
    </Pressable>
  );
}

export function Chip({
  label,
  tone = "neutral",
}: {
  label: string;
  tone?: "neutral" | "ember" | "spark";
}) {
  const palette = {
    neutral: { bg: colors.paper, fg: colors.iron },
    ember: { bg: colors.emberTint, fg: colors.emberInk },
    spark: { bg: colors.magentaTint, fg: colors.magentaInk },
  }[tone];
  return (
    <View style={[styles.chip, { backgroundColor: palette.bg }]}>
      <Text style={[styles.chipText, { color: palette.fg }]}>{label}</Text>
    </View>
  );
}

const styles = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  content: { paddingHorizontal: 16, paddingBottom: 32, gap: 12 },
  header: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "flex-start",
    paddingHorizontal: 4,
    paddingTop: 8,
    paddingBottom: 8,
  },
  title: { fontFamily: font.semibold, fontSize: fontSize.screenTitle, color: colors.obsidian },
  card: {
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: radius.card,
    padding: 20,
    gap: 6,
  },
  heading: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  body: {
    fontFamily: font.regular,
    fontSize: fontSize.body,
    color: colors.graphite,
    lineHeight: 22,
  },
  label: { fontFamily: font.regular, fontSize: fontSize.small },
  button: {
    height: 48,
    borderRadius: radius.button,
    paddingHorizontal: 16,
    alignItems: "center",
    justifyContent: "center",
  },
  buttonPrimary: { backgroundColor: colors.obsidian },
  buttonGhost: { backgroundColor: colors.snow, borderWidth: 1, borderColor: colors.cloud },
  buttonText: { fontFamily: font.medium, fontSize: fontSize.body },
  chip: {
    alignSelf: "flex-start",
    borderRadius: radius.badge,
    paddingHorizontal: 8,
    paddingVertical: 3,
  },
  chipText: { fontFamily: font.medium, fontSize: fontSize.caption },
});
