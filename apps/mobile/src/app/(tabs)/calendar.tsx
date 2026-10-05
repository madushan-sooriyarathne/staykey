import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { useMemo, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, {
  Easing,
  FadeIn,
  FadeInDown,
  SlideInDown,
  SlideOutDown,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { useCalendar, useDeleteBlock } from "@/api/calendar";
import { MonthGrid, MonthNav } from "@/components/calendar";
import { Button, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Hint, IconButton, PropertySwitcher, Segmented } from "@/components/kit";
import { Stripes } from "@/components/stripes";
import { font } from "@/components/ui";
import {
  addDays,
  eachNight,
  formatShort,
  monthGrid,
  monthStart,
  nightsBetween,
  today,
} from "@/data/dates";
import { useBookings, useCan, useFilter, useProperties } from "@/data/hooks";
import { BLOCK_REASON, plural } from "@/data/labels";
import { HOLDING, isOTA, nightlyRate, physicalUnits, relatedUnits } from "@/data/pricing";
import type { Block, Booking, Currency, ISODate, PropertyConfig } from "@/data/types";
import { guestLabel } from "@/features/bookings/rows";
import { haptics } from "@/lib/haptics";

type Night = { booking?: Booking; block?: Block };
type Kind = "direct" | "awaiting" | "ota" | "request" | "block";

function kindOf(n: Night): Kind | null {
  if (n.block && !n.booking) return "block";
  const b = n.booking;
  if (!b) return null;
  if (b.status === "requested") return "request";
  if (isOTA(b)) return "ota";
  if (b.status === "awaiting_payment") return "awaiting";
  return "direct";
}

export function compactPrice(minor: number, currency: Currency): string {
  return currency === "USD" ? `$${Math.round(minor / 100)}` : `${Math.round(minor / 100_000)}k`;
}

/** Nights for one unit, including stays on linked units that take it off sale. */
function nightsFor(
  p: PropertyConfig,
  unitId: string,
  bookings: Booking[],
  blocks: Block[],
  from: ISODate,
  to: ISODate,
) {
  const ids = relatedUnits(p, unitId);
  const map: Record<ISODate, Night> = {};
  for (const b of bookings) {
    if (b.propertyId !== p.id || !HOLDING.includes(b.status) || !ids.includes(b.unitId)) continue;
    if (b.checkOut <= from || b.checkIn >= to) continue;
    for (const d of eachNight(b.checkIn, b.checkOut)) {
      const prev = map[d]?.booking;
      if (!prev || prev.unitId !== unitId) map[d] = { ...map[d], booking: b };
    }
  }
  for (const k of blocks) {
    if (k.propertyId !== p.id || !k.unitIds.some((u) => ids.includes(u))) continue;
    for (const d of eachNight(k.from, k.to)) map[d] = { ...map[d], block: k };
  }
  return map;
}

export default function CalendarScreen() {
  const properties = useProperties();
  const bookings = useBookings();
  const removeBlock = useDeleteBlock();
  const filter = useFilter((s) => s.propertyId);
  const setFilter = useFilter((s) => s.set);
  const can = useCan();
  const property = properties.find((p) => p.id === filter) ?? properties[0];
  const { blocks, overrides } = useCalendar(property?.id);
  const units = property
    ? physicalUnits(property).concat(property.units.filter((u) => u.linkedUnitIds?.length))
    : [];

  const [view, setView] = useState<"month" | "timeline">("month");
  const [month, setMonth] = useState(monthStart(today()));
  const [dir, setDir] = useState<1 | -1>(1);
  const [unitId, setUnitId] = useState<string | undefined>(undefined);
  const [sel, setSel] = useState<{ start: ISODate; end: ISODate | null } | null>(null);
  const [blockTap, setBlockTap] = useState<Block | null>(null);

  const unit = units.find((u) => u.id === unitId) ?? units[0];
  const grid = monthGrid(month);
  const from = grid[0] as ISODate;
  const to = addDays(grid[grid.length - 1] as ISODate, 1);
  const nights = useMemo(
    () => (property && unit ? nightsFor(property, unit.id, bookings, blocks, from, to) : {}),
    [property, unit, bookings, blocks, from, to],
  );

  if (!property || !unit) return null;
  const day = today();
  const canEdit = can("prices");

  function changeMonth(m: ISODate, d: 1 | -1) {
    setDir(d);
    setMonth(m);
  }

  function tap(d: ISODate) {
    const n = nights[d] ?? {};
    if (n.booking && !sel) {
      haptics.select();
      router.push({ pathname: "/booking/[id]", params: { id: n.booking.id } });
      return;
    }
    if (!canEdit) return;
    if (n.block && !sel) {
      haptics.select();
      setBlockTap(n.block);
      return;
    }
    haptics.select();
    setBlockTap(null);
    if (!sel || sel.end || d < sel.start) setSel({ start: d, end: null });
    else setSel({ start: sel.start, end: d });
  }

  const checkOut = sel ? addDays(sel.end ?? sel.start, 1) : null;
  const selNights = sel && checkOut ? nightsBetween(sel.start, checkOut) : 0;
  const params =
    sel && checkOut
      ? { propertyId: property.id, unitId: unit.id, from: sel.start, to: checkOut }
      : null;

  return (
    <SafeAreaView edges={["top"]} style={s.screen}>
      <ScrollView contentContainerStyle={s.content} showsVerticalScrollIndicator={false}>
        <View style={s.header}>
          <Text style={s.title}>Calendar</Text>
          <Button
            compact
            variant="ghost"
            title="Today"
            onPress={() => {
              changeMonth(monthStart(day), month > monthStart(day) ? -1 : 1);
              setSel(null);
            }}
          />
        </View>

        <PropertySwitcher
          properties={properties}
          value={property.id}
          onChange={(id) => {
            setFilter(id);
            setUnitId(undefined);
            setSel(null);
          }}
        />

        <Segmented
          testID="calendar-view"
          options={[
            { id: "month", label: "Month" },
            { id: "timeline", label: "Timeline" },
          ]}
          value={view}
          onChange={(v) => {
            setView(v);
            setSel(null);
          }}
        />

        {view === "month" && units.length > 1 ? (
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: 8 }}
          >
            {units.map((u) => (
              <Pill
                key={u.id}
                label={u.name}
                selected={u.id === unit.id}
                onPress={() => {
                  setUnitId(u.id);
                  setSel(null);
                }}
              />
            ))}
          </ScrollView>
        ) : null}

        {view === "month" ? (
          <Animated.View entering={FadeIn.duration(220)} style={s.card}>
            <MonthNav month={month} onChange={changeMonth} />
            <MonthGrid
              month={month}
              direction={dir}
              renderDay={(d, inMonth) => {
                const n = nights[d] ?? {};
                const kind = kindOf(n);
                const past = d < day;
                const selected = sel && (d === sel.start || d === sel.end);
                const between = sel?.end && d > sel.start && d < sel.end;
                const price =
                  !kind && !past && inMonth && canEdit
                    ? nightlyRate(property, unit.id, d, overrides)
                    : 0;
                const hasOverride = overrides[unit.id]?.[d]?.price != null;
                return (
                  <Pressable
                    accessibilityLabel={`${formatShort(d)}${kind ? `, ${kind}` : ""}`}
                    disabled={!inMonth || (past && !kind)}
                    onPress={() => tap(d)}
                    style={[
                      s.day,
                      kind === "direct" && s.direct,
                      kind === "awaiting" && s.awaiting,
                      kind === "ota" && s.ota,
                      kind === "request" && s.request,
                      kind === "block" && s.blocked,
                      between && s.between,
                      selected && s.selected,
                      !inMonth && { opacity: 0 },
                    ]}
                  >
                    {kind === "block" ? <Stripes /> : null}
                    <Text
                      style={[
                        s.dayText,
                        past && !kind && { color: colors.ash },
                        kind === "direct" && { color: colors.snow, fontFamily: font.medium },
                        kind === "awaiting" && { color: colors.emberInk, fontFamily: font.medium },
                        selected && { color: colors.snow, fontFamily: font.medium },
                        d === day && { fontFamily: font.semibold },
                      ]}
                    >
                      {Number(d.slice(8))}
                    </Text>
                    {price ? (
                      <Text
                        style={[
                          s.price,
                          hasOverride && { color: colors.emberInk },
                          selected && { color: colors.mist },
                        ]}
                      >
                        {compactPrice(price, property.currency)}
                      </Text>
                    ) : null}
                    {d === day ? (
                      <View
                        style={[s.todayDot, kind === "direct" && { backgroundColor: colors.snow }]}
                      />
                    ) : null}
                  </Pressable>
                );
              }}
            />
          </Animated.View>
        ) : (
          <Timeline
            property={property}
            month={month}
            onMonth={changeMonth}
            bookings={bookings}
            blocks={blocks}
          />
        )}

        <View style={s.card}>
          <View style={s.legend}>
            <Legend style={s.direct} label="Direct" />
            <Legend style={s.ota} label="OTA via iCal" />
            <Legend style={s.blocked} label="Blocked" striped />
            <Legend style={s.request} label="Request" />
          </View>
        </View>
        {canEdit ? (
          <Hint center>Tap two dates to add a booking, block nights or change rates.</Hint>
        ) : null}
      </ScrollView>

      {sel && params ? (
        <Animated.View
          entering={SlideInDown.springify().damping(20).stiffness(220)}
          exiting={SlideOutDown.duration(180)}
          style={s.bar}
        >
          <View style={s.barHead}>
            <View style={{ flex: 1 }}>
              <Text style={s.barTitle}>
                {formatShort(sel.start)}
                {sel.end ? ` to ${formatShort(checkOut as ISODate)}` : ""}
              </Text>
              <Text style={s.barSub}>
                {unit.name}, {plural(selNights, "night")}
                {sel.end ? "" : ". Tap another date to extend"}
              </Text>
            </View>
            <IconButton icon={I.close} label="Clear selection" onPress={() => setSel(null)} />
          </View>
          <View style={s.barActions}>
            <View style={{ flex: 1.3 }}>
              <Button
                compact
                title="New booking"
                testID="sel-new"
                onPress={() => {
                  setSel(null);
                  router.push({ pathname: "/booking/new", params });
                }}
              />
            </View>
            <View style={{ flex: 1 }}>
              <Button
                compact
                variant="ghost"
                title="Block"
                testID="sel-block"
                onPress={() => router.push({ pathname: "/range/block", params })}
              />
            </View>
            <View style={{ flex: 1 }}>
              <Button
                compact
                variant="ghost"
                title="Rates"
                testID="sel-rates"
                onPress={() => router.push({ pathname: "/range/rates", params })}
              />
            </View>
          </View>
        </Animated.View>
      ) : blockTap ? (
        <Animated.View
          entering={SlideInDown.springify().damping(20).stiffness(220)}
          exiting={SlideOutDown.duration(180)}
          style={s.bar}
        >
          <View style={s.barHead}>
            <View style={{ flex: 1 }}>
              <Text style={s.barTitle}>Blocked, {BLOCK_REASON[blockTap.reason].toLowerCase()}</Text>
              <Text style={s.barSub}>
                {formatShort(blockTap.from)} to {formatShort(blockTap.to)}
                {blockTap.note ? `. ${blockTap.note}` : ""}
              </Text>
            </View>
            <IconButton icon={I.close} label="Close" onPress={() => setBlockTap(null)} />
          </View>
          <Button
            compact
            variant="ghost"
            title="Open these nights"
            onPress={() => {
              haptics.success();
              removeBlock.mutate({ id: blockTap.id, propertyId: blockTap.propertyId });
              setBlockTap(null);
            }}
          />
        </Animated.View>
      ) : null}
    </SafeAreaView>
  );
}

function Legend({ style, label, striped }: { style: object; label: string; striped?: boolean }) {
  return (
    <View style={s.legendItem}>
      <View style={[s.swatch, style]}>{striped ? <Stripes /> : null}</View>
      <Text style={s.legendText}>{label}</Text>
    </View>
  );
}

const COL = 40;
const UNIT_COL = 88;
const ease = Easing.out(Easing.cubic);

/** Every unit as a row and the month as columns, with stays drawn as bars. */
function Timeline({
  property,
  month,
  onMonth,
  bookings,
  blocks,
}: {
  property: PropertyConfig;
  month: ISODate;
  onMonth: (m: ISODate, d: 1 | -1) => void;
  bookings: Booking[];
  blocks: Block[];
}) {
  const start = monthStart(month);
  const days: ISODate[] = [];
  for (let d = start; d.slice(0, 7) === start.slice(0, 7); d = addDays(d, 1)) days.push(d);
  const end = addDays(days[days.length - 1] as ISODate, 1);
  const day = today();
  const units = physicalUnits(property).concat(
    property.units.filter((u) => u.linkedUnitIds?.length),
  );
  const x = (d: ISODate) => nightsBetween(start, d) * COL;

  return (
    <Animated.View
      key={`${property.id}-${month}`}
      entering={FadeInDown.duration(260).easing(ease)}
      style={s.card}
    >
      <MonthNav month={month} onChange={onMonth} />
      <View style={{ flexDirection: "row", marginTop: 8 }}>
        <View style={{ width: UNIT_COL }}>
          <View style={s.tlHead} />
          {units.map((u) => (
            <View key={u.id} style={s.tlUnit}>
              <Text style={s.tlUnitText} numberOfLines={2}>
                {u.name}
              </Text>
            </View>
          ))}
        </View>
        <ScrollView
          horizontal
          showsHorizontalScrollIndicator={false}
          contentOffset={{ x: Math.max(0, x(day) - COL * 2), y: 0 }}
        >
          <View>
            <View style={[s.tlHead, { flexDirection: "row" }]}>
              {days.map((d) => (
                <View key={d} style={[s.tlDay, d === day && s.tlToday]}>
                  <Text style={s.tlDow}>{"SMTWTFS"[new Date(`${d}T00:00`).getDay()]}</Text>
                  <Text
                    style={[
                      s.tlDate,
                      d === day && { fontFamily: font.semibold, color: colors.obsidian },
                    ]}
                  >
                    {Number(d.slice(8))}
                  </Text>
                </View>
              ))}
            </View>
            {units.map((u) => {
              const rowBookings = bookings.filter(
                (b) =>
                  b.propertyId === property.id &&
                  b.unitId === u.id &&
                  HOLDING.includes(b.status) &&
                  b.checkIn < end &&
                  b.checkOut > start,
              );
              const rowBlocks = blocks.filter(
                (k) =>
                  k.propertyId === property.id &&
                  k.unitIds.includes(u.id) &&
                  k.from < end &&
                  k.to > start,
              );
              return (
                <View key={u.id} style={[s.tlRow, { width: days.length * COL }]}>
                  {days.map((d) => (
                    <View key={d} style={[s.tlCell, d === day && s.tlToday]} />
                  ))}
                  {rowBlocks.map((k) => (
                    <View
                      key={k.id}
                      style={[
                        s.tlBar,
                        s.blocked,
                        { left: x(k.from) + COL / 2, width: nightsBetween(k.from, k.to) * COL - 4 },
                      ]}
                    >
                      <Stripes />
                    </View>
                  ))}
                  {rowBookings.map((b) => {
                    const kind = kindOf({ booking: b });
                    return (
                      <Pressable
                        key={b.id}
                        onPress={() => {
                          haptics.select();
                          router.push({ pathname: "/booking/[id]", params: { id: b.id } });
                        }}
                        style={[
                          s.tlBar,
                          kind === "direct" && s.direct,
                          kind === "awaiting" && s.awaiting,
                          kind === "ota" && s.ota,
                          kind === "request" && s.request,
                          {
                            left: x(b.checkIn) + COL / 2,
                            width: nightsBetween(b.checkIn, b.checkOut) * COL - 4,
                          },
                        ]}
                      >
                        <Text
                          numberOfLines={1}
                          style={[
                            s.tlBarText,
                            kind === "direct" && { color: colors.snow },
                            kind === "awaiting" && { color: colors.emberInk },
                          ]}
                        >
                          {guestLabel(b)}
                        </Text>
                      </Pressable>
                    );
                  })}
                </View>
              );
            })}
          </View>
        </ScrollView>
      </View>
    </Animated.View>
  );
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  content: { paddingHorizontal: 16, paddingBottom: 160, gap: 12 },
  header: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingTop: 8,
    paddingHorizontal: 4,
  },
  title: { fontFamily: font.semibold, fontSize: 30, color: colors.obsidian },
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 14,
    gap: 10,
  },
  day: {
    height: 50,
    borderRadius: 12,
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  },
  dayText: { fontFamily: font.regular, fontSize: 14, color: colors.graphite },
  price: { fontFamily: font.regular, fontSize: 10.5, color: colors.fog, marginTop: 1 },
  direct: { backgroundColor: colors.ember },
  awaiting: { backgroundColor: colors.emberTint },
  ota: { backgroundColor: colors.mist },
  request: { borderWidth: 1.5, borderStyle: "dashed", borderColor: colors.steel },
  blocked: { backgroundColor: colors.paper },
  between: { backgroundColor: colors.cloud },
  selected: { backgroundColor: colors.obsidian },
  todayDot: {
    position: "absolute",
    bottom: 5,
    width: 4,
    height: 4,
    borderRadius: 2,
    backgroundColor: colors.obsidian,
  },
  legend: { flexDirection: "row", flexWrap: "wrap", gap: 14, rowGap: 8 },
  legendItem: { flexDirection: "row", alignItems: "center", gap: 6 },
  swatch: { width: 14, height: 14, borderRadius: 5, overflow: "hidden" },
  legendText: { fontFamily: font.regular, fontSize: 12, color: colors.steel },
  bar: {
    position: "absolute",
    left: 12,
    right: 12,
    bottom: 12,
    backgroundColor: colors.snow,
    borderRadius: 26,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 14,
    gap: 12,
    shadowColor: colors.obsidian,
    shadowOpacity: 0.08,
    shadowRadius: 24,
    shadowOffset: { width: 0, height: 8 },
    elevation: 6,
  },
  barHead: { flexDirection: "row", alignItems: "center", gap: 12 },
  barTitle: { fontFamily: font.semibold, fontSize: 16, color: colors.obsidian },
  barSub: { fontFamily: font.regular, fontSize: 13, color: colors.fog, marginTop: 1 },
  barActions: { flexDirection: "row", gap: 8 },
  tlHead: { height: 44 },
  tlUnit: {
    height: 52,
    justifyContent: "center",
    paddingRight: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
  },
  tlUnitText: { fontFamily: font.medium, fontSize: 13, color: colors.obsidian },
  tlDay: { width: COL, alignItems: "center", justifyContent: "center" },
  tlToday: { backgroundColor: colors.subtle },
  tlDow: { fontFamily: font.regular, fontSize: 11, color: colors.ash },
  tlDate: { fontFamily: font.medium, fontSize: 13, color: colors.steel },
  tlRow: { height: 52, flexDirection: "row", borderTopWidth: 1, borderTopColor: colors.cloud },
  tlCell: { width: COL, borderRightWidth: 1, borderRightColor: colors.subtle },
  tlBar: {
    position: "absolute",
    top: 9,
    height: 34,
    borderRadius: 10,
    justifyContent: "center",
    paddingHorizontal: 8,
    overflow: "hidden",
  },
  tlBarText: { fontFamily: font.medium, fontSize: 12, color: colors.obsidian },
});
