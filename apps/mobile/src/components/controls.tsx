import { colors, radius } from "@staykey/tokens";
import { SymbolView, type SymbolViewProps } from "expo-symbols";
import { forwardRef, type ReactNode, useEffect, useState } from "react";
import {
  ActivityIndicator,
  Platform,
  Pressable,
  type PressableProps,
  type StyleProp,
  StyleSheet,
  Switch,
  Text,
  TextInput,
  type TextInputProps,
  View,
  type ViewStyle,
} from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { font } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { spring } from "@/lib/motion";

const AnimatedPressable = Animated.createAnimatedComponent(Pressable);

/** react-native-web colors the "on" thumb teal unless told otherwise. */
const webSwitchProps = Platform.OS === "web" ? ({ activeThumbColor: colors.snow } as object) : {};

/** Pressable that scales down slightly while pressed. */
export function PressScale({
  children,
  style,
  scaleTo = 0.97,
  onPressIn,
  onPressOut,
  ...props
}: Omit<PressableProps, "style"> & {
  style?: StyleProp<ViewStyle>;
  scaleTo?: number;
  children: ReactNode;
}) {
  const scale = useSharedValue(1);
  const animated = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return (
    <AnimatedPressable
      {...props}
      onPressIn={(e) => {
        scale.value = withSpring(scaleTo, spring.control);
        onPressIn?.(e);
      }}
      onPressOut={(e) => {
        scale.value = withSpring(1, spring.control);
        onPressOut?.(e);
      }}
      style={[style, animated]}
    >
      {children}
    </AnimatedPressable>
  );
}

type ButtonProps = {
  title: string;
  onPress?: () => void;
  disabled?: boolean;
  loading?: boolean;
  variant?: "primary" | "ghost" | "light";
  icon?: SymbolViewProps["name"];
  testID?: string;
  /** Smaller button for cards and two-up rows. */
  compact?: boolean;
};

/** Primary call to action. Obsidian fill per the brand; label changes crossfade. */
export function Button({
  title,
  onPress,
  disabled,
  loading,
  variant = "primary",
  icon,
  testID,
  compact,
}: ButtonProps) {
  const inactive = disabled || loading;
  const fg = variant === "primary" ? colors.snow : colors.obsidian;
  return (
    <PressScale
      testID={testID}
      accessibilityRole="button"
      accessibilityState={{ disabled: inactive }}
      disabled={inactive}
      onPress={() => {
        if (variant === "primary") haptics.press();
        onPress?.();
      }}
      style={[
        styles.button,
        variant === "primary" && styles.primary,
        variant === "ghost" && styles.ghost,
        variant === "light" && styles.light,
        compact && styles.compact,
        disabled && styles.disabled,
      ]}
    >
      {loading ? (
        <ActivityIndicator color={fg} />
      ) : (
        <Animated.View key={title} entering={FadeIn.duration(180)} style={styles.buttonInner}>
          {icon ? <SymbolView name={icon} tintColor={fg} size={compact ? 16 : 18} /> : null}
          <Text
            numberOfLines={1}
            style={[
              styles.buttonText,
              compact && { fontSize: 15 },
              { color: variant === "ghost" ? colors.iron : fg },
            ]}
          >
            {title}
          </Text>
        </Animated.View>
      )}
    </PressScale>
  );
}

export function TextLink({
  title,
  onPress,
  testID,
}: {
  title: string;
  onPress: () => void;
  testID?: string;
}) {
  return (
    <Pressable testID={testID} accessibilityRole="link" onPress={onPress} hitSlop={8}>
      <Text style={styles.link}>{title}</Text>
    </Pressable>
  );
}

type FieldProps = TextInputProps & {
  label?: string;
  prefix?: ReactNode;
  suffix?: ReactNode;
  error?: string | null;
};

/** Text input with label, optional prefix/suffix and an ember focus state. */
export const Field = forwardRef<TextInput, FieldProps>(function Field(
  { label, prefix, suffix, error, style, onFocus, onBlur, ...props },
  ref,
) {
  const [focused, setFocused] = useState(false);
  return (
    <View style={styles.field}>
      {label ? <Text style={styles.label}>{label}</Text> : null}
      <View
        style={[
          styles.inputBox,
          focused && styles.inputFocused,
          error ? styles.inputError : undefined,
        ]}
      >
        {prefix}
        <TextInput
          ref={ref}
          placeholderTextColor={colors.ash}
          style={[styles.input, style]}
          onFocus={(e) => {
            setFocused(true);
            onFocus?.(e);
          }}
          onBlur={(e) => {
            setFocused(false);
            onBlur?.(e);
          }}
          {...props}
        />
        {suffix}
      </View>
      {error ? (
        <Animated.Text
          entering={FadeIn.duration(160)}
          exiting={FadeOut.duration(120)}
          style={styles.error}
        >
          {error}
        </Animated.Text>
      ) : null}
    </View>
  );
});

export function SectionLabel({ children }: { children: ReactNode }) {
  return <Text style={styles.label}>{children}</Text>;
}

/** Selectable card used for single-choice questions. */
export function ChoiceCard({
  title,
  description,
  selected,
  onPress,
  icon,
  badge,
  testID,
}: {
  title: string;
  description?: string;
  selected: boolean;
  onPress: () => void;
  icon?: SymbolViewProps["name"];
  badge?: ReactNode;
  testID?: string;
}) {
  return (
    <PressScale
      testID={testID}
      accessibilityRole="radio"
      accessibilityState={{ selected }}
      scaleTo={0.98}
      onPress={() => {
        if (!selected) haptics.select();
        onPress();
      }}
      style={[styles.choice, selected && styles.choiceOn]}
    >
      <View style={styles.choiceTop}>
        {icon ? (
          <View style={styles.iconBox}>
            <SymbolView name={icon} tintColor={colors.iron} size={20} />
          </View>
        ) : null}
        <Radio on={selected} />
      </View>
      <View style={{ gap: 2 }}>
        <View style={styles.rowBetween}>
          <Text style={styles.choiceTitle}>{title}</Text>
          {badge}
        </View>
        {description ? <Text style={styles.muted}>{description}</Text> : null}
      </View>
    </PressScale>
  );
}

/** Compact single-line option with a radio, for longer lists. */
export function OptionRow({
  title,
  description,
  selected,
  onPress,
  badge,
}: {
  title: string;
  description?: string;
  selected: boolean;
  onPress: () => void;
  badge?: ReactNode;
}) {
  return (
    <PressScale
      accessibilityRole="radio"
      accessibilityState={{ selected }}
      scaleTo={0.985}
      onPress={() => {
        if (!selected) haptics.select();
        onPress();
      }}
      style={[styles.option, selected && styles.choiceOn]}
    >
      <Radio on={selected} />
      <View style={{ flex: 1, gap: 2 }}>
        <View style={styles.rowBetween}>
          <Text style={styles.choiceTitle}>{title}</Text>
          {badge}
        </View>
        {description ? <Text style={styles.muted}>{description}</Text> : null}
      </View>
    </PressScale>
  );
}

export function Radio({ on }: { on: boolean }) {
  const scale = useSharedValue(on ? 1 : 0);
  useEffect(() => {
    scale.value = withSpring(on ? 1 : 0, spring.control);
  }, [on, scale]);
  const dot = useAnimatedStyle(() => ({ transform: [{ scale: scale.value }] }));
  return (
    <View style={[styles.radio, on && { borderColor: colors.obsidian }]}>
      <Animated.View style={[styles.radioDot, dot]} />
    </View>
  );
}

export function Checkbox({
  checked,
  onChange,
  label,
}: {
  checked: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <Pressable
      accessibilityRole="checkbox"
      accessibilityState={{ checked }}
      onPress={() => {
        haptics.select();
        onChange(!checked);
      }}
      style={styles.checkRow}
    >
      <View style={[styles.checkbox, checked && styles.checkboxOn]}>
        {checked ? (
          <Animated.View entering={FadeIn.duration(120)}>
            <SymbolView
              name={{ ios: "checkmark", android: "check", web: "check" }}
              tintColor={colors.snow}
              size={14}
            />
          </Animated.View>
        ) : null}
      </View>
      <Text style={[styles.body, { flex: 1 }]}>{label}</Text>
    </Pressable>
  );
}

export function Pill({
  label,
  selected,
  onPress,
  testID,
}: {
  label: string;
  selected: boolean;
  onPress: () => void;
  testID?: string;
}) {
  return (
    <PressScale
      testID={testID}
      accessibilityRole="button"
      accessibilityState={{ selected }}
      scaleTo={0.95}
      onPress={() => {
        haptics.select();
        onPress();
      }}
      style={[styles.pill, selected && styles.pillOn]}
    >
      {selected ? (
        <SymbolView
          name={{ ios: "checkmark", android: "check", web: "check" }}
          tintColor={colors.snow}
          size={13}
        />
      ) : null}
      <Text style={[styles.pillText, selected && { color: colors.snow }]}>{label}</Text>
    </PressScale>
  );
}

export function Stepper({
  label,
  hint,
  value,
  onChange,
  min = 0,
  max = 99,
}: {
  label: string;
  hint?: string;
  value: number;
  onChange: (v: number) => void;
  min?: number;
  max?: number;
}) {
  const set = (v: number) => {
    const next = Math.min(max, Math.max(min, v));
    if (next !== value) {
      haptics.select();
      onChange(next);
    }
  };
  return (
    <View style={styles.stepperRow}>
      <View style={{ flex: 1 }}>
        <Text style={styles.body}>{label}</Text>
        {hint ? <Text style={styles.muted}>{hint}</Text> : null}
      </View>
      <View style={styles.stepper}>
        <StepButton
          symbol="minus"
          disabled={value <= min}
          onPress={() => set(value - 1)}
          label={`Fewer ${label}`}
        />
        <Animated.Text key={value} entering={FadeIn.duration(140)} style={styles.stepValue}>
          {value}
        </Animated.Text>
        <StepButton
          symbol="plus"
          disabled={value >= max}
          onPress={() => set(value + 1)}
          label={`More ${label}`}
        />
      </View>
    </View>
  );
}

function StepButton({
  symbol,
  disabled,
  onPress,
  label,
}: {
  symbol: "plus" | "minus";
  disabled: boolean;
  onPress: () => void;
  label: string;
}) {
  return (
    <PressScale
      accessibilityRole="button"
      accessibilityLabel={label}
      disabled={disabled}
      scaleTo={0.9}
      onPress={onPress}
      style={[styles.stepButton, disabled && { opacity: 0.35 }]}
    >
      <SymbolView
        name={{
          ios: symbol,
          android: symbol === "plus" ? "add" : "remove",
          web: symbol === "plus" ? "add" : "remove",
        }}
        tintColor={colors.iron}
        size={16}
      />
    </PressScale>
  );
}

export function ToggleRow({
  label,
  hint,
  value,
  onChange,
}: {
  label: string;
  hint?: string;
  value: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <View style={styles.stepperRow}>
      <View style={{ flex: 1 }}>
        <Text style={styles.body}>{label}</Text>
        {hint ? <Text style={styles.muted}>{hint}</Text> : null}
      </View>
      <Switch
        value={value}
        onValueChange={(v) => {
          haptics.select();
          onChange(v);
        }}
        trackColor={{ true: colors.obsidian, false: colors.mist }}
        thumbColor={colors.snow}
        {...webSwitchProps}
        ios_backgroundColor={colors.mist}
      />
    </View>
  );
}

export function Card({ children, style }: { children: ReactNode; style?: StyleProp<ViewStyle> }) {
  return <View style={[styles.card, style]}>{children}</View>;
}

/** Ember-tinted note for things the owner should know before continuing. */
export function InfoNote({
  children,
  icon,
  testID,
}: {
  children: ReactNode;
  icon?: SymbolViewProps["name"];
  testID?: string;
}) {
  return (
    <Animated.View
      testID={testID}
      entering={FadeIn.duration(220)}
      exiting={FadeOut.duration(150)}
      style={styles.info}
    >
      <SymbolView
        name={icon ?? { ios: "calendar", android: "calendar_month", web: "calendar_month" }}
        tintColor={colors.ember}
        size={20}
      />
      <Text style={[styles.body, { flex: 1, fontSize: 14 }]}>{children}</Text>
    </Animated.View>
  );
}

export function Divider() {
  return <View style={styles.divider} />;
}

export const styles = StyleSheet.create({
  button: {
    height: 52,
    borderRadius: radius.button,
    paddingHorizontal: 18,
    alignItems: "center",
    justifyContent: "center",
  },
  primary: { backgroundColor: colors.obsidian },
  compact: { height: 44, paddingHorizontal: 14 },
  ghost: { backgroundColor: colors.snow, borderWidth: 1, borderColor: colors.cloud },
  light: { backgroundColor: colors.snow },
  disabled: { opacity: 0.35 },
  buttonInner: { flexDirection: "row", alignItems: "center", gap: 8 },
  buttonText: { fontFamily: font.medium, fontSize: 16 },
  link: {
    fontFamily: font.medium,
    fontSize: 14,
    color: colors.obsidian,
    textDecorationLine: "underline",
    textAlign: "center",
  },
  field: { gap: 6 },
  label: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
  inputBox: {
    minHeight: 52,
    flexDirection: "row",
    alignItems: "center",
    gap: 10,
    paddingHorizontal: 16,
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    borderRadius: radius.input,
  },
  inputFocused: { borderColor: colors.ember, borderWidth: 1.5 },
  inputError: { borderColor: colors.emberInk },
  input: {
    flex: 1,
    minHeight: 50,
    fontFamily: font.regular,
    fontSize: 16,
    color: colors.graphite,
    paddingVertical: 12,
    // Focus is shown on the box, so drop the browser's own outline on web.
    outlineWidth: 0,
  },
  error: { fontFamily: font.medium, fontSize: 13, color: colors.emberInk },
  choice: {
    flex: 1,
    padding: 16,
    gap: 12,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  choiceOn: { borderColor: colors.obsidian, borderWidth: 1.5 },
  choiceTop: { flexDirection: "row", justifyContent: "space-between", alignItems: "flex-start" },
  choiceTitle: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  option: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 12,
    padding: 16,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  rowBetween: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
  },
  iconBox: {
    width: 40,
    height: 40,
    borderRadius: 14,
    backgroundColor: colors.paper,
    alignItems: "center",
    justifyContent: "center",
  },
  muted: { fontFamily: font.regular, fontSize: 13, color: colors.fog, lineHeight: 18 },
  body: { fontFamily: font.regular, fontSize: 15, color: colors.graphite, lineHeight: 21 },
  radio: {
    width: 22,
    height: 22,
    borderRadius: 11,
    borderWidth: 1.5,
    borderColor: colors.mist,
    alignItems: "center",
    justifyContent: "center",
  },
  radioDot: { width: 11, height: 11, borderRadius: 6, backgroundColor: colors.obsidian },
  checkRow: { flexDirection: "row", alignItems: "flex-start", gap: 12, paddingVertical: 4 },
  checkbox: {
    width: 22,
    height: 22,
    borderRadius: 7,
    borderWidth: 1.5,
    borderColor: colors.mist,
    backgroundColor: colors.snow,
    alignItems: "center",
    justifyContent: "center",
    marginTop: 1,
  },
  checkboxOn: { backgroundColor: colors.obsidian, borderColor: colors.obsidian },
  pill: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingHorizontal: 14,
    paddingVertical: 9,
    borderRadius: radius.pill,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  pillOn: { backgroundColor: colors.obsidian, borderColor: colors.obsidian },
  pillText: { fontFamily: font.medium, fontSize: 14, color: colors.iron },
  stepperRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12 },
  stepper: { flexDirection: "row", alignItems: "center", gap: 10 },
  stepButton: {
    width: 34,
    height: 34,
    borderRadius: 17,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    alignItems: "center",
    justifyContent: "center",
  },
  stepValue: {
    minWidth: 24,
    textAlign: "center",
    fontFamily: font.semibold,
    fontSize: 16,
    color: colors.obsidian,
  },
  card: {
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    borderRadius: radius.card,
    paddingHorizontal: 18,
    paddingVertical: 6,
  },
  info: {
    flexDirection: "row",
    gap: 12,
    padding: 16,
    borderRadius: 24,
    backgroundColor: colors.emberTint,
    alignItems: "flex-start",
  },
  divider: { height: 1, backgroundColor: colors.cloud },
});
