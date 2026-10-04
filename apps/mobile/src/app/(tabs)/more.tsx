import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import type { SymbolViewProps } from "expo-symbols";
import type { ReactNode } from "react";
import { StyleSheet, Text, View } from "react-native";
import { Chevron, IconBox, Tag } from "@/components/brand";
import { PressScale } from "@/components/controls";
import { font, Screen } from "@/components/ui";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";
import { PLANS } from "@/lib/purchases";
import { useSession } from "@/lib/session";

/** Restart and preview shortcuts. On in development, and in builds made for prototype testing. */
const prototypeTools = __DEV__ || process.env.EXPO_PUBLIC_PROTOTYPE_TOOLS === "true";

export default function MoreScreen() {
  const { subscription, role, bookingPageUrl, reset } = useSession();
  const clearDraft = useOnboarding((s) => s.clear);
  const plan =
    subscription.status === "active" ? PLANS.find((p) => p.id === subscription.plan) : undefined;

  return (
    <Screen title="More">
      {role === "owner" ? (
        <Group>
          <Item
            testID="more-subscription"
            icon={{ ios: "creditcard", android: "credit_card", web: "credit_card" }}
            title="Subscription"
            subtitle={
              plan
                ? `${plan.name}, billed ${subscription.status === "active" ? subscription.period : ""}`
                : undefined
            }
            trailing={plan ? undefined : <Tag tone="spark" label="Free until first booking" />}
            onPress={() => router.push("/paywall")}
          />
          <Item
            icon={{ ios: "globe", android: "public", web: "public" }}
            title="Booking page"
            subtitle={bookingPageUrl.replace(/^https?:\/\//, "") || undefined}
          />
          <Item icon={{ ios: "person.2", android: "group", web: "group" }} title="Team" />
          <Item
            icon={{ ios: "bell", android: "notifications", web: "notifications" }}
            title="Notifications"
          />
        </Group>
      ) : (
        <Group>
          <Item
            icon={{ ios: "bell", android: "notifications", web: "notifications" }}
            title="Notifications"
          />
        </Group>
      )}

      {prototypeTools ? (
        <>
          <Text style={styles.section}>Prototype</Text>
          <Group>
            {role === "owner" ? (
              <Item
                icon={{ ios: "sparkles", android: "auto_awesome", web: "auto_awesome" }}
                title="Preview first-booking paywall"
                onPress={() =>
                  router.push({ pathname: "/paywall", params: { reason: "first-booking" } })
                }
              />
            ) : null}
            <Item
              testID="more-restart"
              icon={{ ios: "arrow.counterclockwise", android: "restart_alt", web: "restart_alt" }}
              title="Restart onboarding"
              onPress={() => {
                haptics.warning();
                clearDraft();
                reset();
              }}
            />
          </Group>
        </>
      ) : null}
    </Screen>
  );
}

function Group({ children }: { children: ReactNode }) {
  return <View style={styles.group}>{children}</View>;
}

function Item({
  icon,
  title,
  subtitle,
  trailing,
  onPress,
  testID,
}: {
  icon: SymbolViewProps["name"];
  title: string;
  subtitle?: string;
  trailing?: ReactNode;
  onPress?: () => void;
  testID?: string;
}) {
  return (
    <PressScale
      testID={testID}
      accessibilityRole="button"
      disabled={!onPress}
      scaleTo={0.985}
      onPress={() => {
        haptics.select();
        onPress?.();
      }}
      style={styles.item}
    >
      <IconBox name={icon} size={36} />
      <View style={{ flex: 1, gap: 1 }}>
        <Text style={styles.title}>{title}</Text>
        {subtitle ? (
          <Text style={styles.subtitle} numberOfLines={1}>
            {subtitle}
          </Text>
        ) : null}
      </View>
      {trailing}
      {onPress ? <Chevron /> : <Text style={styles.soon}>Soon</Text>}
    </PressScale>
  );
}

const styles = StyleSheet.create({
  group: {
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: radius.card,
    paddingHorizontal: 14,
    paddingVertical: 4,
  },
  item: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 10 },
  title: { fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  subtitle: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  soon: { fontFamily: font.regular, fontSize: 13, color: colors.ash },
  section: {
    fontFamily: font.medium,
    fontSize: 13,
    color: colors.fog,
    paddingHorizontal: 6,
    paddingTop: 8,
  },
});
