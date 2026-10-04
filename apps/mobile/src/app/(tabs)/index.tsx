import { colors, radius } from "@staykey/tokens";
import { useEffect } from "react";
import { Share, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeInDown,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { Chevron, IconBox, Row, Tag, Tick } from "@/components/brand";
import { Button } from "@/components/controls";
import { font, Label, Screen } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { type Setup, useSession } from "@/lib/session";

const today = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
}).format(new Date());

const ease = Easing.out(Easing.cubic);

type Item = { id: string; title: string; subtitle?: string; done: boolean };

/** Everything that isn't needed for a first booking, in the order that helps most. */
function checklist(setup: Setup): Item[] {
  const items: Item[] = [{ id: "publish", title: "Publish your booking page", done: true }];
  if (setup.otas.length > 0) {
    const names = setup.otas.join(" and ");
    items.push({
      id: "sync",
      title: `Sync your ${names} ${setup.otas.length > 1 ? "calendars" : "calendar"}`,
      subtitle: "Stops double bookings and turns on instant book",
      done: false,
    });
  }
  items.push(
    { id: "widget", title: "Add the website widget", done: false },
    { id: "rates", title: "Add seasonal rates", done: false },
    { id: "team", title: "Invite your caretaker", done: false },
    {
      id: "photos",
      title: setup.photoCount === 0 ? "Add photos" : "Add more photos",
      done: setup.photoCount >= 3,
    },
    { id: "cards", title: "Turn on card payments", done: false },
  );
  return items;
}

export default function TodayScreen() {
  const { role, propertyName, bookingPageUrl, setup } = useSession();

  return (
    <Screen eyebrow={today} title="Today">
      <View style={styles.stats}>
        <Stat value={0} label="Arrivals" />
        <Stat value={0} label="Departures" />
        <Stat value={0} label="In house" />
      </View>

      {role === "owner" ? (
        <>
          <Checklist items={checklist(setup)} />
          <Animated.View
            entering={FadeInDown.delay(160).duration(380).easing(ease)}
            style={styles.card}
          >
            <Text style={styles.cardTitle}>No bookings yet</Text>
            <Text style={styles.muted}>
              Share your link on WhatsApp and Instagram to get your first one.
            </Text>
            <View style={{ paddingTop: 10 }}>
              <Button
                variant="ghost"
                title="Share booking link"
                icon={{ ios: "square.and.arrow.up", android: "share", web: "share" }}
                onPress={() => {
                  haptics.select();
                  Share.share({
                    message: `Book ${propertyName} directly with us: ${bookingPageUrl}`,
                    url: bookingPageUrl,
                  });
                }}
              />
            </View>
          </Animated.View>
        </>
      ) : (
        <Animated.View
          entering={FadeInDown.duration(380).easing(ease)}
          style={[styles.card, styles.teamCard]}
        >
          <IconBox tone="spark" name={{ ios: "person.2", android: "group", web: "group" }} />
          <View style={{ flex: 1, gap: 2 }}>
            <Text style={styles.cardTitle}>You're on the {propertyName} team</Text>
            <Text style={styles.muted}>Arrivals, departures and guest notes show up here.</Text>
          </View>
        </Animated.View>
      )}
    </Screen>
  );
}

function Checklist({ items }: { items: Item[] }) {
  const done = items.filter((i) => i.done).length;
  const nextId = items.find((i) => !i.done)?.id;
  const fill = useSharedValue(0);

  useEffect(() => {
    fill.value = withDelay(250, withTiming(done / items.length, { duration: 700, easing: ease }));
  }, [done, fill, items.length]);

  const bar = useAnimatedStyle(() => ({ width: `${fill.value * 100}%` }));

  return (
    <Animated.View entering={FadeInDown.duration(380).easing(ease)} style={styles.card}>
      <View style={styles.cardHead}>
        <Text style={styles.cardTitle}>Finish setting up</Text>
        <Label tone="faint">
          {done} of {items.length} done
        </Label>
      </View>
      <View style={styles.track}>
        <Animated.View style={[styles.fill, bar]} />
      </View>
      {items.map((item, i) => (
        <View key={item.id} style={i > 0 ? styles.divided : undefined}>
          <Row
            leading={<Tick done={item.done} />}
            title={item.title}
            subtitle={item.done ? undefined : item.subtitle}
            muted={item.done}
            trailing={
              item.done ? null : item.id === nextId ? (
                <Tag tone="ember" label="Next" />
              ) : (
                <Chevron />
              )
            }
          />
        </View>
      ))}
    </Animated.View>
  );
}

function Stat({ value, label }: { value: number; label: string }) {
  return (
    <View style={styles.stat}>
      <Text style={styles.statValue}>{value}</Text>
      <Label>{label}</Label>
    </View>
  );
}

const styles = StyleSheet.create({
  stats: { flexDirection: "row", gap: 8 },
  stat: {
    flex: 1,
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: 24,
    paddingHorizontal: 16,
    paddingVertical: 14,
  },
  statValue: { fontFamily: font.semibold, fontSize: 28, color: colors.obsidian },
  card: {
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: radius.card,
    paddingHorizontal: 18,
    paddingVertical: 16,
    gap: 4,
  },
  teamCard: { flexDirection: "row", alignItems: "center", gap: 12 },
  cardHead: { flexDirection: "row", alignItems: "center", justifyContent: "space-between" },
  cardTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  muted: { fontFamily: font.regular, fontSize: 14, lineHeight: 20, color: colors.steel },
  track: {
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.cloud,
    overflow: "hidden",
    marginTop: 8,
    marginBottom: 4,
  },
  fill: { height: 4, borderRadius: 2, backgroundColor: colors.ember },
  divided: { borderTopWidth: 1, borderTopColor: colors.cloud },
});
