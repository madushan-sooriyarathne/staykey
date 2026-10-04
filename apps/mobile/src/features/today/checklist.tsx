import { colors } from "@staykey/tokens";
import { router } from "expo-router";
import { useEffect } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeInDown,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { Chevron, Tag, Tick } from "@/components/brand";
import { font } from "@/components/ui";
import { CHANNEL_LABEL } from "@/data/labels";
import type { PropertyConfig, TeamMember } from "@/data/types";
import { haptics } from "@/lib/haptics";

type Item = { id: string; title: string; subtitle?: string; done: boolean; href?: string };

const ease = Easing.out(Easing.cubic);

/** Everything that isn't needed for a first booking, in the order that helps most. */
export function checklistFor(p: PropertyConfig, team: TeamMember[]): Item[] {
  const base = `/property/${p.id}`;
  const items: Item[] = [{ id: "publish", title: "Publish your booking page", done: true }];
  if (p.ical.length > 0) {
    const names = p.ical.map((f) => CHANNEL_LABEL[f.channel]).join(" and ");
    items.push({
      id: "sync",
      title: `Sync your ${names} ${p.ical.length > 1 ? "calendars" : "calendar"}`,
      subtitle: "Stops double bookings and turns on instant book",
      done: p.ical.every((f) => f.url && f.status === "ok"),
      href: `${base}/ical`,
    });
  }
  items.push(
    { id: "widget", title: "Add the website widget", done: false, href: `${base}/share` },
    { id: "rates", title: "Add seasonal rates", done: p.seasons.length > 0, href: `${base}/rates` },
    {
      id: "team",
      title: "Invite your caretaker",
      done: team.some((m) => m.role === "caretaker"),
      href: "/team",
    },
    {
      id: "photos",
      title: p.photos.length === 0 ? "Add photos" : "Add more photos",
      done: p.photos.length >= 3,
      href: `${base}/photos`,
    },
    {
      id: "cards",
      title: "Turn on card payments",
      done: p.payments.cards === "on",
      href: `${base}/payments`,
    },
  );
  return items;
}

export function Checklist({ property, team }: { property: PropertyConfig; team: TeamMember[] }) {
  const items = checklistFor(property, team);
  const done = items.filter((i) => i.done).length;
  const nextId = items.find((i) => !i.done)?.id;
  const fill = useSharedValue(0);

  useEffect(() => {
    fill.value = withDelay(250, withTiming(done / items.length, { duration: 700, easing: ease }));
  }, [done, fill, items.length]);

  const bar = useAnimatedStyle(() => ({ width: `${fill.value * 100}%` }));
  if (done === items.length) return null;

  return (
    <Animated.View entering={FadeInDown.duration(380).easing(ease)} style={s.card}>
      <View style={s.head}>
        <Text style={s.title}>Finish setting up</Text>
        <Text style={s.meta}>
          {done} of {items.length} done
        </Text>
      </View>
      <View style={s.track}>
        <Animated.View style={[s.fill, bar]} />
      </View>
      {items.map((item, i) => (
        <Pressable
          key={item.id}
          disabled={item.done || !item.href}
          onPress={() => {
            haptics.select();
            if (item.href) router.push(item.href as never);
          }}
          style={({ pressed }) => [s.row, i > 0 && s.divided, pressed && { opacity: 0.6 }]}
        >
          <Tick done={item.done} />
          <View style={{ flex: 1, gap: 2 }}>
            <Text style={[s.rowTitle, item.done && { color: colors.fog }]}>{item.title}</Text>
            {item.subtitle && !item.done ? <Text style={s.rowSub}>{item.subtitle}</Text> : null}
          </View>
          {item.done ? null : item.id === nextId ? <Tag tone="ember" label="Next" /> : <Chevron />}
        </Pressable>
      ))}
    </Animated.View>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: 28,
    paddingHorizontal: 16,
    paddingTop: 16,
    paddingBottom: 4,
  },
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  title: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  meta: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  track: {
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.cloud,
    overflow: "hidden",
    marginTop: 10,
    marginBottom: 6,
  },
  fill: { height: 4, borderRadius: 2, backgroundColor: colors.ember },
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12 },
  divided: { borderTopWidth: 1, borderTopColor: colors.cloud },
  rowTitle: { fontFamily: font.regular, fontSize: 15, color: colors.graphite },
  rowSub: { fontFamily: font.regular, fontSize: 13, lineHeight: 18, color: colors.fog },
});
