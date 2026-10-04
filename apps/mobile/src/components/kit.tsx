import { formatMoney } from "@staykey/api-client";
import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { SymbolView, type SymbolViewProps } from "expo-symbols";
import { Children, Fragment, isValidElement, type ReactNode, useEffect, useState } from "react";
import {
  KeyboardAvoidingView,
  type LayoutChangeEvent,
  Modal,
  Platform,
  Pressable,
  ScrollView,
  type StyleProp,
  StyleSheet,
  Switch,
  Text,
  View,
  type ViewStyle,
} from "react-native";
import Animated, {
  Easing,
  FadeIn,
  FadeInDown,
  FadeOut,
  useAnimatedStyle,
  useSharedValue,
  withSpring,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { balanceOf, isOTA } from "@/data/pricing";
import type { Booking, Currency, PropertyConfig } from "@/data/types";
import { haptics } from "@/lib/haptics";
import { spring } from "@/lib/motion";
import { Chevron, IconBox, Tag } from "./brand";
import { PressScale } from "./controls";
import { I } from "./icons";
import { font } from "./ui";

export const money = (amount: number, currency: Currency) => formatMoney(amount, currency);

const ease = Easing.out(Easing.cubic);

/** Staggered entrance for list content. Keep `index` small; later items appear together. */
export function Appear({
  index = 0,
  children,
  style,
}: {
  index?: number;
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
}) {
  return (
    <Animated.View
      entering={FadeInDown.delay(Math.min(index, 6) * 45)
        .duration(320)
        .easing(ease)}
      style={style}
    >
      {children}
    </Animated.View>
  );
}

export function IconButton({
  icon,
  onPress,
  tone = "default",
  dot,
  label,
  testID,
}: {
  icon: SymbolViewProps["name"];
  onPress: () => void;
  tone?: "default" | "ember";
  dot?: boolean;
  label: string;
  testID?: string;
}) {
  const ember = tone === "ember";
  return (
    <PressScale
      testID={testID}
      accessibilityRole="button"
      accessibilityLabel={label}
      hitSlop={6}
      scaleTo={0.92}
      onPress={() => {
        haptics.select();
        onPress();
      }}
      style={[s.iconButton, ember && { backgroundColor: colors.ember, borderColor: colors.ember }]}
    >
      <SymbolView name={icon} tintColor={ember ? colors.snow : colors.iron} size={18} />
      {dot ? <View style={s.dot} /> : null}
    </PressScale>
  );
}

type Action = { label: string; onPress: () => void; disabled?: boolean; testID?: string };

/** Top bar for pushed screens: round back button, centered title, optional text action. */
export function AppBar({
  title,
  action,
  right,
  close,
  onBack,
}: {
  title?: string;
  action?: Action;
  right?: ReactNode;
  close?: boolean;
  onBack?: () => void;
}) {
  return (
    <View style={s.appBar}>
      <IconButton
        testID="appbar-back"
        icon={close ? I.close : I.back}
        label={close ? "Close" : "Back"}
        onPress={() => {
          if (onBack) onBack();
          else if (router.canGoBack()) router.back();
          else router.replace("/");
        }}
      />
      <Text style={s.appBarTitle} numberOfLines={1}>
        {title}
      </Text>
      <View style={s.appBarRight}>
        {action ? (
          <Pressable
            testID={action.testID}
            hitSlop={10}
            disabled={action.disabled}
            onPress={() => {
              haptics.select();
              action.onPress();
            }}
          >
            <Text style={[s.appBarAction, action.disabled && { color: colors.ash }]}>
              {action.label}
            </Text>
          </Pressable>
        ) : (
          right
        )}
      </View>
    </View>
  );
}

/** A pushed screen: app bar, scrolling content and an optional pinned footer. */
export function Page({
  title,
  action,
  right,
  close,
  footer,
  children,
  scroll = true,
  contentStyle,
}: {
  title?: string;
  action?: Action;
  right?: ReactNode;
  close?: boolean;
  footer?: ReactNode;
  children: ReactNode;
  scroll?: boolean;
  contentStyle?: StyleProp<ViewStyle>;
}) {
  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <KeyboardAvoidingView
        behavior={Platform.OS === "ios" ? "padding" : undefined}
        style={{ flex: 1 }}
      >
        <AppBar title={title} action={action} right={right} close={close} />
        {scroll ? (
          <ScrollView
            contentContainerStyle={[s.content, contentStyle]}
            keyboardShouldPersistTaps="handled"
            showsVerticalScrollIndicator={false}
          >
            {children}
          </ScrollView>
        ) : (
          <View style={[{ flex: 1 }, contentStyle]}>{children}</View>
        )}
        {footer ? <View style={s.footer}>{footer}</View> : null}
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

/** Body of a form sheet route. Native sheets draw their own grabber; web and Android get a close button. */
export function SheetPage({
  title,
  subtitle,
  children,
}: {
  title: string;
  subtitle?: string;
  children: ReactNode;
}) {
  return (
    <ScrollView
      style={{ backgroundColor: colors.snow }}
      contentContainerStyle={s.sheet}
      keyboardShouldPersistTaps="handled"
      showsVerticalScrollIndicator={false}
    >
      <View style={s.sheetHead}>
        <View style={{ flex: 1, gap: 2 }}>
          <Text style={s.sheetTitle}>{title}</Text>
          {subtitle ? <Text style={s.muted}>{subtitle}</Text> : null}
        </View>
        {Platform.OS !== "ios" ? (
          <IconButton icon={I.close} label="Close" onPress={() => router.back()} />
        ) : null}
      </View>
      {children}
    </ScrollView>
  );
}

export function SectionHeader({
  title,
  meta,
  action,
}: {
  title: string;
  meta?: string;
  action?: Action;
}) {
  return (
    <View style={s.sectionHeader}>
      <Text style={s.sectionTitle}>{title}</Text>
      {meta ? <Text style={s.meta}>{meta}</Text> : null}
      {action ? (
        <Pressable hitSlop={8} onPress={action.onPress}>
          <Text style={s.sectionAction}>{action.label}</Text>
        </Pressable>
      ) : null}
    </View>
  );
}

/** White card holding rows, with hairline dividers between them. */
export function List({
  children,
  title,
  meta,
  style,
}: {
  children: ReactNode;
  title?: string;
  meta?: ReactNode;
  style?: StyleProp<ViewStyle>;
}) {
  const items = Children.toArray(children).filter((c) => isValidElement(c));
  return (
    <View style={[s.list, style]}>
      {title ? (
        <View style={s.listTitle}>
          <Text style={s.cardTitle}>{title}</Text>
          {typeof meta === "string" ? <Text style={s.meta}>{meta}</Text> : meta}
        </View>
      ) : null}
      {items.map((child, i) => (
        <Fragment key={isValidElement(child) && child.key != null ? child.key : i}>
          {i > 0 || title ? <View style={s.divider} /> : null}
          {child}
        </Fragment>
      ))}
    </View>
  );
}

export function ListRow({
  icon,
  iconTone,
  leading,
  title,
  subtitle,
  value,
  trailing,
  onPress,
  chevron = !!onPress,
  destructive,
  muted,
  wrap,
  testID,
}: {
  icon?: SymbolViewProps["name"];
  iconTone?: "neutral" | "ember" | "spark";
  leading?: ReactNode;
  title: string;
  subtitle?: string;
  value?: string;
  trailing?: ReactNode;
  onPress?: () => void;
  chevron?: boolean;
  destructive?: boolean;
  muted?: boolean;
  wrap?: boolean;
  testID?: string;
}) {
  const body = (
    <>
      {leading ?? (icon ? <IconBox name={icon} tone={iconTone} size={36} /> : null)}
      <View style={{ flex: 1, gap: 2 }}>
        <Text
          numberOfLines={wrap ? undefined : 1}
          style={[
            s.rowTitle,
            destructive && { color: colors.emberInk },
            muted && { color: colors.fog },
          ]}
        >
          {title}
        </Text>
        {subtitle ? (
          <Text numberOfLines={wrap ? undefined : 2} style={s.rowSub}>
            {subtitle}
          </Text>
        ) : null}
      </View>
      {value ? <Text style={s.rowValue}>{value}</Text> : null}
      {trailing}
      {chevron ? <Chevron /> : null}
    </>
  );
  if (!onPress) return <View style={s.row}>{body}</View>;
  return (
    <Pressable
      testID={testID}
      accessibilityRole="button"
      onPress={() => {
        haptics.select();
        onPress();
      }}
      style={({ pressed }) => [s.row, pressed && { opacity: 0.6 }]}
    >
      {body}
    </Pressable>
  );
}

/** A switch row that saves as it changes. */
export function SwitchRow({
  title,
  subtitle,
  value,
  onChange,
  icon,
}: {
  title: string;
  subtitle?: string;
  value: boolean;
  onChange: (v: boolean) => void;
  icon?: SymbolViewProps["name"];
}) {
  return (
    <ListRow
      icon={icon}
      title={title}
      subtitle={subtitle}
      chevron={false}
      trailing={<BrandSwitch value={value} onChange={onChange} label={title} />}
    />
  );
}

const webSwitch = Platform.OS === "web" ? ({ activeThumbColor: colors.snow } as object) : {};

export function BrandSwitch({
  value,
  onChange,
  label,
}: {
  value: boolean;
  onChange: (v: boolean) => void;
  label: string;
}) {
  return (
    <Switch
      accessibilityLabel={label}
      value={value}
      onValueChange={(v) => {
        haptics.select();
        onChange(v);
      }}
      trackColor={{ true: colors.obsidian, false: colors.mist }}
      thumbColor={colors.snow}
      ios_backgroundColor={colors.mist}
      {...webSwitch}
    />
  );
}

/** Label and value on one line, for price breakdowns and settings summaries. */
export function KV({
  label,
  value,
  total,
  muted,
  tone,
}: {
  label: string;
  value: ReactNode;
  total?: boolean;
  muted?: boolean;
  tone?: "ember" | "spark";
}) {
  return (
    <View style={[s.kv, total && s.kvTotal]}>
      <Text style={[s.kvLabel, total && s.kvStrong, muted && { color: colors.fog }]}>{label}</Text>
      {typeof value === "string" ? (
        <Text
          style={[
            s.kvValue,
            total && s.kvStrong,
            muted && { color: colors.fog },
            tone === "ember" && { color: colors.emberInk },
            tone === "spark" && { color: colors.magentaInk },
          ]}
        >
          {value}
        </Text>
      ) : (
        value
      )}
    </View>
  );
}

export function Card({
  children,
  style,
  title,
  meta,
}: {
  children: ReactNode;
  style?: StyleProp<ViewStyle>;
  title?: string;
  meta?: ReactNode;
}) {
  return (
    <View style={[s.card, style]}>
      {title ? (
        <View style={s.cardHead}>
          <Text style={s.cardTitle}>{title}</Text>
          {typeof meta === "string" ? <Text style={s.meta}>{meta}</Text> : meta}
        </View>
      ) : null}
      {children}
    </View>
  );
}

/** Animated segmented control. The thumb springs to the chosen option. */
export function Segmented<T extends string>({
  options,
  value,
  onChange,
  testID,
}: {
  options: { id: T; label: string; count?: number }[];
  value: T;
  onChange: (v: T) => void;
  testID?: string;
}) {
  const [width, setWidth] = useState(0);
  const index = Math.max(
    0,
    options.findIndex((o) => o.id === value),
  );
  const x = useSharedValue(index);

  useEffect(() => {
    x.value = withSpring(index, spring.control);
  }, [index, x]);

  const seg = (width - 8) / options.length;
  const thumb = useAnimatedStyle(() => ({
    width: seg,
    transform: [{ translateX: x.value * seg }],
  }));

  return (
    <View
      testID={testID}
      style={s.segmented}
      onLayout={(e: LayoutChangeEvent) => setWidth(e.nativeEvent.layout.width)}
    >
      {width > 0 ? <Animated.View style={[s.segThumb, thumb]} /> : null}
      {options.map((o) => (
        <Pressable
          key={o.id}
          testID={testID ? `${testID}-${o.id}` : undefined}
          accessibilityRole="tab"
          accessibilityState={{ selected: value === o.id }}
          style={s.segOption}
          onPress={() => {
            if (value !== o.id) haptics.select();
            onChange(o.id);
          }}
        >
          <Text numberOfLines={1} style={[s.segText, value === o.id && s.segTextOn]}>
            {o.label}
          </Text>
          {o.count ? (
            <View style={s.segCount}>
              <Text style={s.segCountText}>{o.count}</Text>
            </View>
          ) : null}
        </Pressable>
      ))}
    </View>
  );
}

export function initials(name: string): string {
  const parts = name.trim().split(/\s+/).filter(Boolean);
  if (!parts.length) return "";
  return (
    (parts[0]?.[0] ?? "") + (parts.length > 1 ? (parts[parts.length - 1]?.[0] ?? "") : "")
  ).toUpperCase();
}

export function Avatar({
  name,
  size = 40,
  tone = "neutral",
}: {
  name: string;
  size?: number;
  tone?: "neutral" | "dark" | "ember";
}) {
  const palette = {
    neutral: { bg: colors.paper, fg: colors.iron },
    dark: { bg: colors.graphite, fg: colors.snow },
    ember: { bg: colors.emberTint, fg: colors.emberInk },
  }[tone];
  const text = initials(name);
  return (
    <View
      style={[
        s.avatar,
        { width: size, height: size, borderRadius: size / 2, backgroundColor: palette.bg },
      ]}
    >
      {text ? (
        <Text style={[s.avatarText, { color: palette.fg, fontSize: size * 0.36 }]}>{text}</Text>
      ) : (
        <SymbolView name={I.person} tintColor={palette.fg} size={size * 0.5} />
      )}
    </View>
  );
}

export function EmptyState({
  icon,
  title,
  body,
  action,
}: {
  icon: SymbolViewProps["name"];
  title: string;
  body?: string;
  action?: ReactNode;
}) {
  return (
    <Animated.View entering={FadeIn.duration(260)} style={s.empty}>
      <IconBox name={icon} size={48} />
      <Text style={s.emptyTitle}>{title}</Text>
      {body ? <Text style={[s.muted, { textAlign: "center" }]}>{body}</Text> : null}
      {action}
    </Animated.View>
  );
}

const STATUS: Record<
  Booking["status"],
  { label: string; tone: "ink" | "dash" | "ember" | "spark" | "soft" }
> = {
  requested: { label: "Requested", tone: "dash" },
  awaiting_payment: { label: "Awaiting payment", tone: "ember" },
  confirmed: { label: "Confirmed", tone: "ink" },
  checked_in: { label: "In house", tone: "spark" },
  checked_out: { label: "Checked out", tone: "soft" },
  cancelled: { label: "Cancelled", tone: "soft" },
  declined: { label: "Declined", tone: "soft" },
};

export function StatusTag({ booking }: { booking: Booking }) {
  const st = STATUS[booking.status];
  return <Tag label={st.label} tone={st.tone} />;
}

/** "Paid", "$180 due" or "iCal", the money state owners scan for. */
export function PaymentTag({ booking, currency }: { booking: Booking; currency: Currency }) {
  if (isOTA(booking)) return <Tag label="iCal" tone="soft" />;
  if (booking.status === "requested") return <Tag label="Requested" tone="dash" />;
  if (booking.status === "cancelled" || booking.status === "declined")
    return <Tag label={STATUS[booking.status].label} tone="soft" />;
  const due = balanceOf(booking);
  if (due === 0) return <Tag label="Paid" tone="spark" />;
  return <Tag label={`${money(due, currency)} due`} tone="ember" />;
}

/** Pill that switches between properties, with "All properties" where it makes sense. */
export function PropertySwitcher({
  properties,
  value,
  onChange,
  allowAll,
}: {
  properties: PropertyConfig[];
  value: string | "all";
  onChange: (id: string | "all") => void;
  allowAll?: boolean;
}) {
  const [open, setOpen] = useState(false);
  if (properties.length < 2) return null;
  const label =
    value === "all"
      ? "All properties"
      : (properties.find((p) => p.id === value)?.name ?? "All properties");
  const options = [
    ...(allowAll ? [{ id: "all", name: "All properties" }] : []),
    ...properties.map((p) => ({ id: p.id, name: p.name })),
  ];

  return (
    <>
      <PressScale
        testID="property-switcher"
        accessibilityRole="button"
        scaleTo={0.96}
        onPress={() => {
          haptics.select();
          setOpen(true);
        }}
        style={s.switcher}
      >
        <SymbolView name={I.house} tintColor={colors.iron} size={14} />
        <Text style={s.switcherText} numberOfLines={1}>
          {label}
        </Text>
        <SymbolView name={I.chevronDown} tintColor={colors.fog} size={12} />
      </PressScale>
      <Modal transparent visible={open} animationType="fade" onRequestClose={() => setOpen(false)}>
        <Pressable style={s.scrim} onPress={() => setOpen(false)}>
          <Animated.View
            entering={FadeInDown.duration(220).easing(ease)}
            exiting={FadeOut.duration(120)}
            style={s.menu}
          >
            {options.map((o, i) => (
              <Fragment key={o.id}>
                {i > 0 ? <View style={s.divider} /> : null}
                <Pressable
                  style={s.menuRow}
                  onPress={() => {
                    haptics.select();
                    onChange(o.id);
                    setOpen(false);
                  }}
                >
                  <Text style={[s.rowTitle, o.id === value && { fontFamily: font.medium }]}>
                    {o.name}
                  </Text>
                  {o.id === value ? (
                    <SymbolView name={I.check} tintColor={colors.obsidian} size={16} />
                  ) : null}
                </Pressable>
              </Fragment>
            ))}
          </Animated.View>
        </Pressable>
      </Modal>
    </>
  );
}

export type MenuItem = {
  label: string;
  icon: SymbolViewProps["name"];
  onPress: () => void;
  destructive?: boolean;
  testID?: string;
};

/** Small action menu that drops in from the top, for "more" buttons. */
export function Menu({
  visible,
  onClose,
  items,
}: {
  visible: boolean;
  onClose: () => void;
  items: MenuItem[];
}) {
  return (
    <Modal transparent visible={visible} animationType="fade" onRequestClose={onClose}>
      <Pressable style={[s.scrim, { paddingTop: 96 }]} onPress={onClose}>
        <Animated.View
          entering={FadeInDown.duration(220).easing(ease)}
          style={[s.menu, { marginLeft: "auto", width: 260 }]}
        >
          {items.map((item, i) => (
            <Fragment key={item.label}>
              {i > 0 ? <View style={s.divider} /> : null}
              <Pressable
                testID={item.testID}
                style={s.menuRow}
                onPress={() => {
                  haptics.select();
                  onClose();
                  item.onPress();
                }}
              >
                <Text style={[s.rowTitle, item.destructive && { color: colors.emberInk }]}>
                  {item.label}
                </Text>
                <SymbolView
                  name={item.icon}
                  tintColor={item.destructive ? colors.emberInk : colors.iron}
                  size={17}
                />
              </Pressable>
            </Fragment>
          ))}
        </Animated.View>
      </Pressable>
    </Modal>
  );
}

/** Two buttons side by side, for card and footer actions. */
export function Two({ children }: { children: ReactNode }) {
  return (
    <View style={s.two}>
      {Children.toArray(children).map((c, i) => (
        <View key={isValidElement(c) && c.key != null ? c.key : i} style={{ flex: 1 }}>
          {c}
        </View>
      ))}
    </View>
  );
}

export function Hint({ children, center }: { children: ReactNode; center?: boolean }) {
  return <Text style={[s.hint, center && { textAlign: "center" }]}>{children}</Text>;
}

export const ui = StyleSheet.create({
  muted: { fontFamily: font.regular, fontSize: 14, lineHeight: 20, color: colors.steel },
  faint: { fontFamily: font.regular, fontSize: 13, lineHeight: 18, color: colors.fog },
  body: { fontFamily: font.regular, fontSize: 15, lineHeight: 21, color: colors.graphite },
  strong: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  title: { fontFamily: font.semibold, fontSize: 24, lineHeight: 30, color: colors.obsidian },
  amount: { fontFamily: font.semibold, fontSize: 15, color: colors.obsidian },
});

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  content: { paddingHorizontal: 16, paddingTop: 4, paddingBottom: 28, gap: 12 },
  footer: { flexDirection: "row", gap: 8, paddingHorizontal: 16, paddingTop: 10, paddingBottom: 8 },
  iconButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    alignItems: "center",
    justifyContent: "center",
  },
  dot: {
    position: "absolute",
    top: 7,
    right: 8,
    width: 9,
    height: 9,
    borderRadius: 5,
    backgroundColor: colors.ember,
    borderWidth: 2,
    borderColor: colors.snow,
  },
  appBar: {
    height: 56,
    flexDirection: "row",
    alignItems: "center",
    paddingHorizontal: 16,
    gap: 12,
  },
  appBarTitle: {
    flex: 1,
    textAlign: "center",
    fontFamily: font.semibold,
    fontSize: 17,
    color: colors.obsidian,
  },
  appBarRight: { minWidth: 40, alignItems: "flex-end" },
  appBarAction: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  sheet: { padding: 20, paddingBottom: 32, gap: 14 },
  sheetHead: {
    flexDirection: "row",
    alignItems: "flex-start",
    gap: 12,
    paddingTop: Platform.OS === "ios" ? 8 : 0,
  },
  sheetTitle: { fontFamily: font.semibold, fontSize: 22, color: colors.obsidian },
  muted: { fontFamily: font.regular, fontSize: 14, lineHeight: 20, color: colors.steel },
  meta: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  sectionHeader: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 4,
    paddingTop: 8,
  },
  sectionTitle: { flex: 1, fontFamily: font.medium, fontSize: 13, color: colors.fog },
  sectionAction: { fontFamily: font.medium, fontSize: 13, color: colors.obsidian },
  list: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    paddingHorizontal: 16,
    paddingVertical: 4,
  },
  listTitle: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingTop: 14,
    paddingBottom: 10,
  },
  divider: { height: 1, backgroundColor: colors.cloud },
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12, minHeight: 56 },
  rowTitle: { fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  rowSub: { fontFamily: font.regular, fontSize: 13, lineHeight: 18, color: colors.fog },
  rowValue: { fontFamily: font.regular, fontSize: 14, color: colors.steel },
  kv: {
    flexDirection: "row",
    justifyContent: "space-between",
    alignItems: "center",
    gap: 12,
    paddingVertical: 6,
  },
  kvTotal: { borderTopWidth: 1, borderTopColor: colors.cloud, marginTop: 6, paddingTop: 12 },
  kvLabel: { flex: 1, fontFamily: font.regular, fontSize: 14, color: colors.steel },
  kvValue: { fontFamily: font.regular, fontSize: 14, color: colors.graphite },
  kvStrong: { fontFamily: font.semibold, fontSize: 16, color: colors.obsidian },
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 4,
  },
  cardHead: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
    marginBottom: 6,
  },
  cardTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  segmented: {
    flexDirection: "row",
    height: 44,
    padding: 4,
    borderRadius: radius.button + 2,
    backgroundColor: colors.cloud,
  },
  segThumb: {
    position: "absolute",
    top: 4,
    left: 4,
    bottom: 4,
    borderRadius: radius.button - 2,
    backgroundColor: colors.snow,
  },
  segOption: {
    flex: 1,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "center",
    gap: 5,
  },
  segText: { fontFamily: font.medium, fontSize: 13.5, color: colors.steel },
  segTextOn: { color: colors.obsidian },
  segCount: {
    minWidth: 18,
    height: 18,
    borderRadius: 9,
    paddingHorizontal: 5,
    backgroundColor: colors.ember,
    alignItems: "center",
    justifyContent: "center",
  },
  segCountText: { fontFamily: font.semibold, fontSize: 11, color: colors.snow },
  avatar: { alignItems: "center", justifyContent: "center" },
  avatarText: { fontFamily: font.semibold },
  empty: { alignItems: "center", gap: 8, paddingVertical: 32, paddingHorizontal: 24 },
  emptyTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian, marginTop: 4 },
  switcher: {
    flexDirection: "row",
    alignItems: "center",
    alignSelf: "flex-start",
    gap: 8,
    height: 36,
    paddingHorizontal: 14,
    borderRadius: 18,
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    maxWidth: "100%",
  },
  switcherText: { fontFamily: font.medium, fontSize: 14, color: colors.obsidian, flexShrink: 1 },
  scrim: { flex: 1, backgroundColor: "rgba(9,9,11,0.28)", paddingTop: 120, paddingHorizontal: 16 },
  menu: {
    backgroundColor: colors.snow,
    borderRadius: 24,
    paddingHorizontal: 16,
    paddingVertical: 4,
    borderWidth: 1,
    borderColor: colors.cloud,
  },
  menuRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingVertical: 15,
  },
  two: { flexDirection: "row", gap: 8 },
  hint: {
    fontFamily: font.regular,
    fontSize: 13,
    lineHeight: 18,
    color: colors.fog,
    paddingHorizontal: 4,
  },
});
