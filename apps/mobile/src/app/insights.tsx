import { colors, radius } from "@staykey/tokens";
import { useEffect, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeIn,
  useAnimatedStyle,
  useSharedValue,
  withDelay,
  withTiming,
} from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Appear, Card, money, Page, PropertySwitcher, Segmented, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { addMonths, eachNight, formatMonthShort, monthStart, today } from "@/data/dates";
import { useFilter, useProperties } from "@/data/hooks";
import { HOLDING, isOTA, occupancy } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Booking, Source } from "@/data/types";

const ease = Easing.out(Easing.cubic);

type Period = { from: string; to: string; label: string };

/** Revenue, occupancy, average nightly rate and booking sources by month or year. */
export default function Insights() {
  const properties = useProperties();
  const all = useData((s) => s.bookings);
  const filter = useFilter((s) => s.propertyId);
  const setFilter = useFilter((s) => s.set);
  const [view, setView] = useState<"month" | "year">("month");
  const scope = filter === "all" ? properties : properties.filter((p) => p.id === filter);
  const currency = scope[0]?.currency ?? "USD";
  const ids = new Set(scope.filter((p) => p.currency === currency).map((p) => p.id));
  const bookings = all.filter(
    (b) => ids.has(b.propertyId) && HOLDING.includes(b.status) && b.status !== "requested",
  );

  const now = monthStart(today());
  const year = `${now.slice(0, 4)}-01-01`;
  const current: Period =
    view === "month"
      ? { from: now, to: addMonths(now, 1), label: formatLong(now) }
      : { from: year, to: `${Number(now.slice(0, 4)) + 1}-01-01`, label: now.slice(0, 4) };
  const previous: Period =
    view === "month"
      ? { from: addMonths(now, -1), to: now, label: formatLong(addMonths(now, -1)) }
      : { from: `${Number(now.slice(0, 4)) - 1}-01-01`, to: year, label: "" };

  const revenue = (p: Period) =>
    bookings
      .filter((b) => !isOTA(b) && b.checkIn >= p.from && b.checkIn < p.to)
      .reduce((n, b) => n + b.total, 0);
  const rev = revenue(current);
  const prev = revenue(previous);
  const change = prev > 0 ? Math.round(((rev - prev) / prev) * 100) : null;

  const bars =
    view === "month"
      ? Array.from({ length: 6 }, (_, i) => addMonths(now, i - 5)).map((m) => ({
          month: m,
          label: formatMonthShort(m),
          value: revenue({ from: m, to: addMonths(m, 1), label: "" }),
          current: m === now,
        }))
      : Array.from({ length: 12 }, (_, i) => addMonths(year, i)).map((m) => ({
          month: m,
          label: formatMonthShort(m)[0] ?? "",
          value: revenue({ from: m, to: addMonths(m, 1), label: "" }),
          current: m === now,
        }));
  const max = Math.max(1, ...bars.map((b) => b.value));

  const occ = scope.length
    ? scope.reduce((n, p) => n + occupancy(p, bookings, current.from, current.to), 0) / scope.length
    : 0;
  const directNights = bookings
    .filter((b) => !isOTA(b))
    .flatMap((b) =>
      eachNight(b.checkIn, b.checkOut)
        .filter((d) => d >= current.from && d < current.to)
        .map(() => b),
    );
  const avg = directNights.length
    ? Math.round(
        directNights.reduce(
          (n, b) => n + b.total / Math.max(1, eachNight(b.checkIn, b.checkOut).length),
          0,
        ) /
          directNights.length /
          100,
      ) * 100
    : 0;

  const sources = sourceShare(bookings, current);

  return (
    <Page title="Insights">
      <Segmented
        options={[
          { id: "month", label: "Month" },
          { id: "year", label: "Year" },
        ]}
        value={view}
        onChange={setView}
      />
      <PropertySwitcher properties={properties} value={filter} onChange={setFilter} allowAll />

      <Appear index={0}>
        <Card>
          <Text style={ui.faint}>Revenue booked for {current.label}</Text>
          <Animated.Text key={`${view}-${filter}`} entering={FadeIn.duration(260)} style={s.big}>
            {money(rev, currency)}
          </Animated.Text>
          {change != null ? (
            <Tag
              tone={change >= 0 ? "spark" : "soft"}
              label={`${change >= 0 ? "Up" : "Down"} ${Math.abs(change)}% on ${view === "month" ? previous.label.split(" ")[0] : "last year"}`}
            />
          ) : null}
          <View style={s.bars}>
            {bars.map((b, i) => (
              <Bar
                key={`${view}-${filter}-${b.month}`}
                share={b.value / max}
                current={b.current}
                label={b.label}
                delay={i * 50}
              />
            ))}
          </View>
        </Card>
      </Appear>

      <Appear index={1} style={{ flexDirection: "row", gap: 8 }}>
        <View style={s.stat}>
          <Text style={s.statValue}>{Math.round(occ * 100)}%</Text>
          <Text style={ui.faint}>Occupancy</Text>
        </View>
        <View style={s.stat}>
          <Text style={s.statValue}>{avg ? money(avg, currency) : "None"}</Text>
          <Text style={ui.faint}>Average night</Text>
        </View>
      </Appear>

      <Appear index={2}>
        <Card title="Where bookings came from">
          {sources.length ? (
            <>
              <View style={s.stack}>
                {sources.map((x, i) => (
                  <Segment key={x.label} share={x.share} color={x.color} delay={200 + i * 80} />
                ))}
              </View>
              <View style={s.legend}>
                {sources.map((x) => (
                  <View key={x.label} style={s.legendItem}>
                    <View style={[s.swatch, { backgroundColor: x.color }]} />
                    <Text style={s.legendText}>
                      {x.label} {Math.round(x.share * 100)}%
                    </Text>
                  </View>
                ))}
              </View>
            </>
          ) : (
            <Text style={ui.muted}>No stays in this period yet.</Text>
          )}
        </Card>
      </Appear>
      <Text style={[ui.faint, { textAlign: "center" }]}>
        Revenue counts direct and manual bookings by arrival date. OTA payouts stay in their
        dashboards.
      </Text>
    </Page>
  );
}

function formatLong(month: string) {
  return new Date(`${month}T00:00`).toLocaleString("en-GB", { month: "long" });
}

function sourceShare(bookings: Booking[], p: Period) {
  const groups: { label: string; color: string; match: (s: Source) => boolean }[] = [
    { label: "Direct", color: colors.ember, match: (s) => s === "page" || s === "widget" },
    { label: "Booking.com", color: colors.steel, match: (s) => s === "booking" },
    { label: "Airbnb", color: colors.ash, match: (s) => s === "airbnb" },
    {
      label: "WhatsApp",
      color: colors.mist,
      match: (s) => ["whatsapp", "phone", "walkin", "other"].includes(s),
    },
  ];
  const nights = (b: Booking) =>
    eachNight(b.checkIn, b.checkOut).filter((d) => d >= p.from && d < p.to).length;
  const totals = groups.map((g) =>
    bookings.filter((b) => g.match(b.source)).reduce((n, b) => n + nights(b), 0),
  );
  const sum = totals.reduce((a, b) => a + b, 0);
  if (!sum) return [];
  return groups
    .map((g, i) => ({ label: g.label, color: g.color, share: (totals[i] ?? 0) / sum }))
    .filter((x) => x.share > 0);
}

function Bar({
  share,
  current,
  label,
  delay,
}: {
  share: number;
  current: boolean;
  label: string;
  delay: number;
}) {
  const h = useSharedValue(0);
  useEffect(() => {
    h.value = withDelay(delay, withTiming(Math.max(0.04, share), { duration: 600, easing: ease }));
  }, [share, delay, h]);
  const style = useAnimatedStyle(() => ({ height: `${h.value * 100}%` }));
  return (
    <View style={s.barCol}>
      <View style={s.barTrack}>
        <Animated.View
          style={[s.bar, { backgroundColor: current ? colors.ember : colors.mist }, style]}
        />
      </View>
      <Text style={[s.barLabel, current && { color: colors.obsidian, fontFamily: font.medium }]}>
        {label}
      </Text>
    </View>
  );
}

function Segment({ share, color, delay }: { share: number; color: string; delay: number }) {
  const w = useSharedValue(0);
  useEffect(() => {
    w.value = withDelay(delay, withTiming(share, { duration: 600, easing: ease }));
  }, [share, delay, w]);
  const style = useAnimatedStyle(() => ({ width: `${w.value * 100}%` }));
  return <Animated.View style={[{ height: "100%", backgroundColor: color }, style]} />;
}

const s = StyleSheet.create({
  big: { fontFamily: font.semibold, fontSize: 34, color: colors.obsidian, marginVertical: 4 },
  bars: { flexDirection: "row", gap: 8, height: 150, marginTop: 16, alignItems: "flex-end" },
  barCol: { flex: 1, height: "100%", gap: 6 },
  barTrack: { flex: 1, justifyContent: "flex-end" },
  bar: { width: "100%", borderRadius: 8 },
  barLabel: { textAlign: "center", fontFamily: font.regular, fontSize: 11, color: colors.fog },
  stat: {
    flex: 1,
    backgroundColor: colors.snow,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
  },
  statValue: { fontFamily: font.semibold, fontSize: 24, color: colors.obsidian },
  stack: {
    flexDirection: "row",
    height: 14,
    borderRadius: 7,
    overflow: "hidden",
    backgroundColor: colors.cloud,
    gap: 2,
    marginVertical: 8,
  },
  legend: { flexDirection: "row", flexWrap: "wrap", gap: 14, rowGap: 8 },
  legendItem: { flexDirection: "row", alignItems: "center", gap: 6 },
  swatch: { width: 12, height: 12, borderRadius: 4 },
  legendText: { fontFamily: font.regular, fontSize: 12.5, color: colors.steel },
  card: { borderRadius: radius.card },
});
