import { colors } from "@staykey/tokens";
import { router } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn, FadeOut } from "react-native-reanimated";
import { IconBox } from "@/components/brand";
import { I } from "@/components/icons";
import { Appear, EmptyState, List, ListRow, Page, Segmented } from "@/components/kit";
import { font } from "@/components/ui";
import { dayLabel, formatClock } from "@/data/dates";
import { useData } from "@/data/store";
import type { Activity, ActivityKind } from "@/data/types";

type Tab = "all" | "bookings" | "payments" | "sync";

const KINDS: Record<Exclude<Tab, "all">, ActivityKind[]> = {
  bookings: ["request", "booking", "cancellation"],
  payments: ["slip", "payment"],
  sync: ["sync", "import"],
};

const ICON: Record<ActivityKind, keyof typeof I> = {
  request: "clock",
  booking: "calendar",
  cancellation: "close",
  slip: "doc",
  payment: "cash",
  sync: "warning",
  import: "sync",
};

/** Every alert in one feed: bookings, requests, slips, cancellations and sync problems. */
export default function ActivityScreen() {
  const activity = useData((s) => s.activity);
  const markRead = useData((s) => s.markRead);
  const [tab, setTab] = useState<Tab>("all");
  const items = tab === "all" ? activity : activity.filter((a) => KINDS[tab].includes(a.kind));
  const unread = activity.some((a) => !a.read);

  const groups: { label: string; items: Activity[] }[] = [];
  for (const a of items) {
    const label = dayLabel(a.at);
    const g = groups.find((x) => x.label === label);
    if (g) g.items.push(a);
    else groups.push({ label, items: [a] });
  }

  function open(a: Activity) {
    markRead(a.id);
    if (a.bookingId) router.push({ pathname: "/booking/[id]", params: { id: a.bookingId } });
    else if (a.kind === "sync" && a.propertyId)
      router.push({ pathname: "/property/[id]/ical", params: { id: a.propertyId } });
  }

  return (
    <Page
      title="Activity"
      action={{
        label: "Mark read",
        onPress: () => markRead(),
        disabled: !unread,
        testID: "activity-read",
      }}
    >
      <Segmented
        options={[
          { id: "all", label: "All" },
          { id: "bookings", label: "Bookings" },
          { id: "payments", label: "Payments" },
          { id: "sync", label: "Sync" },
        ]}
        value={tab}
        onChange={setTab}
      />
      <Animated.View key={tab} entering={FadeIn.duration(220)} style={{ gap: 12 }}>
        {groups.length === 0 ? (
          <EmptyState
            icon={I.activity}
            title="Nothing here yet"
            body="Alerts for this kind of event will show up here."
          />
        ) : null}
        {groups.map((g, i) => (
          <Appear key={g.label} index={i} style={{ gap: 6 }}>
            <Text style={s.section}>{g.label}</Text>
            <List>
              {g.items.map((a) => (
                <ListRow
                  key={a.id}
                  leading={
                    <View>
                      <IconBox
                        name={I[ICON[a.kind]]}
                        tone={
                          a.kind === "sync" || (a.kind === "request" && !a.read)
                            ? "ember"
                            : "neutral"
                        }
                        size={36}
                      />
                      {!a.read ? (
                        <Animated.View exiting={FadeOut.duration(200)} style={s.dot} />
                      ) : null}
                    </View>
                  }
                  title={a.title}
                  subtitle={a.subtitle}
                  wrap
                  chevron={false}
                  trailing={<Text style={s.time}>{formatClock(a.at)}</Text>}
                  onPress={a.bookingId || a.kind === "sync" ? () => open(a) : () => markRead(a.id)}
                />
              ))}
            </List>
          </Appear>
        ))}
      </Animated.View>
    </Page>
  );
}

const s = StyleSheet.create({
  section: {
    fontFamily: font.medium,
    fontSize: 13,
    color: colors.fog,
    paddingHorizontal: 4,
    paddingTop: 4,
  },
  dot: {
    position: "absolute",
    top: -2,
    right: -2,
    width: 11,
    height: 11,
    borderRadius: 6,
    backgroundColor: colors.ember,
    borderWidth: 2,
    borderColor: colors.snow,
  },
  time: {
    fontFamily: font.regular,
    fontSize: 12,
    color: colors.fog,
    alignSelf: "flex-start",
    paddingTop: 2,
  },
});
