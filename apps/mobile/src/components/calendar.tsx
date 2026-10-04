import { colors } from "@staykey/tokens";
import { type ReactNode, useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeInLeft, FadeInRight } from "react-native-reanimated";
import {
  addMonths,
  formatMonth,
  inSameMonth,
  monthGrid,
  monthStart,
  nightsBetween,
  today,
} from "@/data/dates";
import type { ISODate } from "@/data/types";
import { haptics } from "@/lib/haptics";
import { I } from "./icons";
import { IconButton } from "./kit";
import { font } from "./ui";

export const WEEKDAYS = ["M", "T", "W", "T", "F", "S", "S"];

export function MonthNav({
  month,
  onChange,
  right,
}: {
  month: ISODate;
  onChange: (m: ISODate, dir: 1 | -1) => void;
  right?: ReactNode;
}) {
  return (
    <View style={s.nav}>
      <Text style={s.navTitle}>{formatMonth(month)}</Text>
      {right}
      <View style={{ flexDirection: "row", gap: 8 }}>
        <IconButton
          icon={I.chevronLeft}
          label="Previous month"
          onPress={() => onChange(addMonths(month, -1), -1)}
        />
        <IconButton
          icon={I.chevronRight}
          label="Next month"
          onPress={() => onChange(addMonths(month, 1), 1)}
        />
      </View>
    </View>
  );
}

/**
 * Month grid with a slide when the month changes. `renderDay` draws each cell so the main
 * calendar and the date pickers share one layout.
 */
export function MonthGrid({
  month,
  direction,
  renderDay,
}: {
  month: ISODate;
  direction: 1 | -1;
  renderDay: (date: ISODate, inMonth: boolean) => ReactNode;
}) {
  const days = monthGrid(month);
  const weeks: ISODate[][] = [];
  for (let i = 0; i < days.length; i += 7) weeks.push(days.slice(i, i + 7));
  const entering = (direction > 0 ? FadeInRight : FadeInLeft).duration(240);
  return (
    <View>
      <View style={s.week}>
        {WEEKDAYS.map((d, i) => (
          // biome-ignore lint/suspicious/noArrayIndexKey: weekday initials repeat
          <Text key={i} style={s.weekday}>
            {d}
          </Text>
        ))}
      </View>
      <Animated.View key={month} entering={entering} style={{ gap: 4 }}>
        {weeks.map((w) => (
          <View key={w[0]} style={s.week}>
            {w.map((d) => (
              <View key={d} style={{ flex: 1 }}>
                {renderDay(d, inSameMonth(d, month))}
              </View>
            ))}
          </View>
        ))}
      </Animated.View>
    </View>
  );
}

/** Check-in and check-out picker. The first tap sets arrival, the second sets departure. */
export function RangePicker({
  start,
  end,
  onChange,
  isBlocked,
  allowPast,
}: {
  start: ISODate | null;
  end: ISODate | null;
  onChange: (start: ISODate | null, end: ISODate | null) => void;
  isBlocked?: (night: ISODate) => boolean;
  allowPast?: boolean;
}) {
  const [month, setMonth] = useState(monthStart(start ?? today()));
  const [dir, setDir] = useState<1 | -1>(1);
  const now = today();

  function tap(d: ISODate) {
    haptics.select();
    if (!start || end || d <= start) {
      onChange(d, null);
      return;
    }
    onChange(start, d);
  }

  return (
    <View style={{ gap: 10 }}>
      <MonthNav
        month={month}
        onChange={(m, d) => {
          setDir(d);
          setMonth(m);
        }}
      />
      <MonthGrid
        month={month}
        direction={dir}
        renderDay={(d, inMonth) => {
          const past = !allowPast && d < now;
          const isStart = d === start;
          const isEnd = d === end;
          const between = !!start && !!end && d > start && d < end;
          const blocked = isBlocked?.(d) ?? false;
          const disabled = past || !inMonth;
          return (
            <Pressable
              disabled={disabled}
              accessibilityLabel={d}
              onPress={() => tap(d)}
              style={[
                s.day,
                between && s.between,
                (isStart || isEnd) && s.endpoint,
                blocked && !isStart && !isEnd && s.blocked,
              ]}
            >
              <Text
                style={[
                  s.dayText,
                  !inMonth && { color: "transparent" },
                  past && inMonth && { color: colors.mist },
                  blocked && { color: colors.ash, textDecorationLine: "line-through" },
                  (isStart || isEnd) && { color: colors.snow, fontFamily: font.medium },
                  d === now && !isStart && !isEnd && { fontFamily: font.semibold },
                ]}
              >
                {Number(d.slice(8))}
              </Text>
            </Pressable>
          );
        }}
      />
      {start && end ? (
        <Text style={s.summary}>
          {nightsBetween(start, end)} {nightsBetween(start, end) === 1 ? "night" : "nights"}
        </Text>
      ) : (
        <Text style={s.summary}>
          {start ? "Now pick the check-out day" : "Pick the check-in day"}
        </Text>
      )}
    </View>
  );
}

const s = StyleSheet.create({
  nav: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 8 },
  navTitle: { flex: 1, fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  week: { flexDirection: "row", gap: 4 },
  weekday: {
    flex: 1,
    textAlign: "center",
    fontFamily: font.medium,
    fontSize: 12,
    color: colors.fog,
    paddingVertical: 6,
  },
  day: { height: 42, borderRadius: 12, alignItems: "center", justifyContent: "center" },
  dayText: { fontFamily: font.regular, fontSize: 14, color: colors.graphite },
  between: { backgroundColor: colors.cloud },
  endpoint: { backgroundColor: colors.obsidian },
  blocked: { backgroundColor: colors.paper },
  summary: { fontFamily: font.regular, fontSize: 13, color: colors.fog, textAlign: "center" },
});
