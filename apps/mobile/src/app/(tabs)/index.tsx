import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { useMemo } from "react";
import { Share, StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeInDown, FadeOut, LinearTransition } from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { Glow } from "@/components/glow";
import { I } from "@/components/icons";
import { Appear, IconButton, List, money, PropertySwitcher, Two, ui } from "@/components/kit";
import { font, Screen } from "@/components/ui";
import { addDays, formatLong, formatRange, formatShort, nightsBetween, today } from "@/data/dates";
import { useFilter, useProperties } from "@/data/hooks";
import { clock, plural } from "@/data/labels";
import { balanceOf } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Booking, PropertyConfig } from "@/data/types";
import { GuestRow, guestLabel, placeLabel } from "@/features/bookings/rows";
import { Checklist } from "@/features/today/checklist";
import { openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

const ease = Easing.out(Easing.cubic);

export default function TodayScreen() {
  const role = useSession((s) => s.role);
  return role === "caretaker" ? <CaretakerToday /> : <OwnerToday />;
}

function useScoped() {
  const properties = useProperties();
  const bookings = useData((s) => s.bookings);
  const filter = useFilter((s) => s.propertyId);
  const setFilter = useFilter((s) => s.set);
  const scoped = filter === "all" ? bookings : bookings.filter((b) => b.propertyId === filter);
  const byId = useMemo(() => Object.fromEntries(properties.map((p) => [p.id, p])), [properties]);
  return {
    properties,
    bookings: scoped,
    filter,
    setFilter,
    byId,
    multi: properties.length > 1 && filter === "all",
  };
}

function OwnerToday() {
  const { properties, bookings, filter, setFilter, byId, multi } = useScoped();
  const team = useData((s) => s.team);
  const unread = useData((s) => s.activity.some((a) => !a.read));
  const setStatus = useData((s) => s.setStatus);
  const day = today();

  const live = bookings.filter((b) => !["cancelled", "declined"].includes(b.status));
  const arrivals = live.filter((b) => b.checkIn === day && b.status !== "requested");
  const departures = live.filter((b) => b.checkOut === day && b.status !== "requested");
  const inHouse = live.filter(
    (b) => b.checkIn <= day && b.checkOut > day && ["confirmed", "checked_in"].includes(b.status),
  );
  const requests = live
    .filter((b) => b.status === "requested")
    .sort((a, b) => (a.requestExpiresAt ?? "").localeCompare(b.requestExpiresAt ?? ""));
  const due = live
    .filter(
      (b) =>
        balanceOf(b) > 0 &&
        b.checkIn <= addDays(day, 14) &&
        b.status !== "requested" &&
        b.checkIn !== day,
    )
    .sort((a, b) => a.checkIn.localeCompare(b.checkIn));
  const focus = filter === "all" ? properties[0] : byId[filter];

  return (
    <Screen
      eyebrow={formatLong(day)}
      title="Today"
      right={
        <View style={{ flexDirection: "row", gap: 8 }}>
          <IconButton
            testID="today-activity"
            icon={I.bell}
            label="Activity"
            dot={unread}
            onPress={() => router.push("/activity")}
          />
          <IconButton
            testID="today-new"
            icon={I.plus}
            tone="ember"
            label="New booking"
            onPress={() => router.push("/booking/new")}
          />
        </View>
      }
    >
      <PropertySwitcher properties={properties} value={filter} onChange={setFilter} allowAll />

      <View style={s.stats}>
        <Stat
          value={arrivals.length}
          label={arrivals.length === 1 ? "Arrival" : "Arrivals"}
          index={0}
        />
        <Stat
          value={departures.length}
          label={departures.length === 1 ? "Departure" : "Departures"}
          index={1}
        />
        <Stat value={inHouse.length} label="In house" index={2} />
      </View>

      {requests.length > 0 ? (
        <RequestCard
          booking={requests[0] as Booking}
          property={byId[(requests[0] as Booking).propertyId]}
          more={requests.length - 1}
          total={requests.length}
          onApprove={(b) => {
            haptics.success();
            setStatus(b.id, "awaiting_payment");
          }}
          onDecline={(b) => {
            haptics.warning();
            setStatus(b.id, "declined");
          }}
        />
      ) : null}

      {focus ? <Checklist property={focus} team={team} /> : null}

      {arrivals.length > 0 ? (
        <Appear index={1}>
          <List title="Arrivals" meta={`${arrivals.length} today`}>
            {arrivals.map((b) => (
              <GuestRow
                key={b.id}
                booking={b}
                property={byId[b.propertyId]}
                subtitle={`${placeLabel(byId[b.propertyId], b, multi)}, ${clock(byId[b.propertyId]?.checkIn ?? "14:00")}`}
              />
            ))}
          </List>
        </Appear>
      ) : null}

      {departures.length > 0 ? (
        <Appear index={2}>
          <List title="Departures" meta={`${departures.length} today`}>
            {departures.map((b) => (
              <GuestRow
                key={b.id}
                booking={b}
                property={byId[b.propertyId]}
                subtitle={`${placeLabel(byId[b.propertyId], b, multi)}, leaves by ${clock(byId[b.propertyId]?.checkOut ?? "11:00")}`}
              />
            ))}
          </List>
        </Appear>
      ) : null}

      {due.length > 0 ? (
        <Appear index={3}>
          <List title="Balances due" meta="Next 14 days">
            {due.slice(0, 4).map((b) => {
              const p = byId[b.propertyId];
              return (
                <GuestRow
                  key={b.id}
                  booking={b}
                  property={p}
                  subtitle={`Arrives ${formatShort(b.checkIn)}${b.slip?.status === "pending" ? ", slip to check" : ""}`}
                  trailing={
                    <Tag tone="ember" label={`${money(balanceOf(b), p?.currency ?? "USD")} due`} />
                  }
                />
              );
            })}
          </List>
        </Appear>
      ) : null}

      {bookings.length === 0 && focus ? (
        <Appear index={2} style={s.card}>
          <Text style={s.cardTitle}>No bookings yet</Text>
          <Text style={ui.muted}>
            Share your link on WhatsApp and Instagram to get your first one.
          </Text>
          <View style={{ paddingTop: 10 }}>
            <Button
              variant="ghost"
              compact
              title="Share booking link"
              icon={I.share}
              onPress={() =>
                Share.share({
                  message: `Book ${focus.name} directly with us: ${focus.bookingPageUrl}`,
                })
              }
            />
          </View>
        </Appear>
      ) : arrivals.length === 0 && departures.length === 0 && requests.length === 0 ? (
        <Appear index={2} style={s.card}>
          <Text style={s.cardTitle}>A quiet day</Text>
          <Text style={ui.muted}>No arrivals, departures or requests today.</Text>
        </Appear>
      ) : null}
    </Screen>
  );
}

function RequestCard({
  booking,
  property,
  more,
  total,
  onApprove,
  onDecline,
}: {
  booking: Booking;
  property?: PropertyConfig;
  more: number;
  total: number;
  onApprove: (b: Booking) => void;
  onDecline: (b: Booking) => void;
}) {
  const hours = booking.requestExpiresAt
    ? Math.max(
        1,
        Math.round((new Date(booking.requestExpiresAt).getTime() - Date.now()) / 3_600_000),
      )
    : 24;
  const guests = booking.adults + booking.children;
  return (
    <Animated.View
      entering={FadeInDown.duration(380).easing(ease)}
      layout={LinearTransition.springify().damping(20)}
      style={s.dark}
    >
      <Glow size={280} style={{ right: -120, top: -130 }} intensity={0.8} />
      <View style={s.darkHead}>
        <Text style={s.darkTitle}>{plural(total, "booking request")}</Text>
        <Tag tone="ember" label="Reply needed" />
      </View>
      <Animated.View
        key={booking.id}
        entering={FadeInDown.duration(300).easing(ease)}
        exiting={FadeOut.duration(150)}
        style={{ gap: 3 }}
      >
        <Text
          style={s.darkName}
          onPress={() => router.push({ pathname: "/booking/[id]", params: { id: booking.id } })}
        >
          {guestLabel(booking)}
        </Text>
        <Text style={s.darkSub}>{placeLabel(property, booking, true)}</Text>
        <Text style={s.darkSub}>
          {formatRange(booking.checkIn, booking.checkOut)}, {plural(guests, "guest")}
          {property ? `, ${money(booking.total, property.currency)}` : ""}
        </Text>
      </Animated.View>
      <View style={{ paddingTop: 12 }}>
        <Two>
          <Button
            compact
            variant="ghost"
            title="Decline"
            onPress={() => onDecline(booking)}
            testID="request-decline"
          />
          <Button
            compact
            variant="light"
            title="Approve"
            onPress={() => onApprove(booking)}
            testID="request-approve"
          />
        </Two>
      </View>
      <Text style={s.darkHint}>
        Auto-declines in {plural(hours, "hour")}.
        {more > 0 ? ` ${more} more ${more === 1 ? "request" : "requests"} waiting.` : ""}
      </Text>
    </Animated.View>
  );
}

function Stat({ value, label, index }: { value: number; label: string; index: number }) {
  return (
    <Appear index={index} style={s.stat}>
      <Animated.Text key={value} entering={FadeInDown.duration(260)} style={s.statValue}>
        {value}
      </Animated.Text>
      <Text style={s.statLabel}>{label}</Text>
    </Appear>
  );
}

/** Caretakers see who is arriving and leaving, with notes and contact, and no prices. */
function CaretakerToday() {
  const { properties, bookings, filter, setFilter, byId, multi } = useScoped();
  const setStatus = useData((s) => s.setStatus);
  const day = today();
  const live = bookings.filter((b) =>
    ["confirmed", "checked_in", "awaiting_payment"].includes(b.status),
  );
  const arrivals = live.filter((b) => b.checkIn === day);
  const departures = live.filter((b) => b.checkOut === day);
  const week = live
    .flatMap((b) => [
      { b, date: b.checkIn, kind: "arrives" as const },
      { b, date: b.checkOut, kind: "leaves" as const },
    ])
    .filter((e) => e.date > day && e.date <= addDays(day, 7))
    .sort((a, b) => a.date.localeCompare(b.date));

  return (
    <Screen
      eyebrow={formatLong(day)}
      title="Today"
      right={<IconButton icon={I.bell} label="Activity" onPress={() => router.push("/activity")} />}
    >
      <PropertySwitcher properties={properties} value={filter} onChange={setFilter} allowAll />
      <View style={s.stats}>
        <Stat
          value={arrivals.length}
          label={arrivals.length === 1 ? "Arrival" : "Arrivals"}
          index={0}
        />
        <Stat value={departures.length} label="Departures" index={1} />
      </View>

      {arrivals.map((b, i) => {
        const p = byId[b.propertyId];
        const extras = b.extras
          .map((id) => p?.extras.find((e) => e.id === id)?.name)
          .filter(Boolean) as string[];
        return (
          <Appear key={b.id} index={i + 1} style={s.card}>
            <GuestRow
              booking={b}
              showMoney={false}
              subtitle={
                multi
                  ? `${placeLabel(p, b, true)}, arriving ${clock(p?.checkIn ?? "14:00")}`
                  : `Arriving ${clock(p?.checkIn ?? "14:00")}, ${plural(b.adults, "adult")}`
              }
              trailing={
                b.status === "checked_in" ? <Tag tone="spark" label="In house" /> : undefined
              }
            />
            {b.guestNote ? (
              <View style={s.note}>
                <Text style={s.noteLabel}>Guest note</Text>
                <Text style={ui.body}>{b.guestNote}</Text>
              </View>
            ) : null}
            {extras.length ? (
              <View style={s.chips}>
                {extras.map((e) => (
                  <Tag key={e} tone="soft" label={`${e} booked`} />
                ))}
              </View>
            ) : null}
            <View style={{ paddingTop: 12 }}>
              <Two>
                <Button
                  compact
                  variant="ghost"
                  title="WhatsApp"
                  icon={I.chat}
                  onPress={() => openWhatsApp(b.guest.phone)}
                />
                {b.status === "checked_in" ? (
                  <Button compact variant="ghost" title="Checked in" icon={I.check} disabled />
                ) : (
                  <Button
                    compact
                    title="Check in"
                    onPress={() => {
                      haptics.success();
                      setStatus(b.id, "checked_in");
                    }}
                  />
                )}
              </Two>
            </View>
          </Appear>
        );
      })}

      {arrivals.length === 0 ? (
        <Appear index={1} style={s.card}>
          <Text style={s.cardTitle}>No arrivals today</Text>
          <Text style={ui.muted}>Upcoming stays for the week are below.</Text>
        </Appear>
      ) : null}

      <Appear index={3}>
        <List title="Next 7 days">
          {week.length === 0 ? (
            <Text style={[ui.muted, { paddingVertical: 14 }]}>Nothing scheduled this week.</Text>
          ) : null}
          {week.map(({ b, date, kind }) => (
            <GuestRow
              key={`${b.id}-${kind}`}
              booking={b}
              showMoney={false}
              subtitle={
                kind === "arrives"
                  ? `${formatShort(date)}, arrives for ${plural(nightsBetween(b.checkIn, b.checkOut), "night")}`
                  : `${formatShort(date)}, leaves by ${clock(byId[b.propertyId]?.checkOut ?? "11:00")}`
              }
              trailing={<Tag tone="soft" label={kind === "arrives" ? "Arrives" : "Leaves"} />}
            />
          ))}
        </List>
      </Appear>
    </Screen>
  );
}

const s = StyleSheet.create({
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
  statLabel: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
  card: {
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: radius.card,
    paddingHorizontal: 16,
    paddingVertical: 14,
    gap: 4,
  },
  cardTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  dark: {
    backgroundColor: colors.graphite,
    borderRadius: radius.card,
    padding: 18,
    gap: 8,
    overflow: "hidden",
  },
  darkHead: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
    marginBottom: 4,
  },
  darkTitle: { fontFamily: font.semibold, fontSize: 17, color: colors.snow },
  darkName: { fontFamily: font.medium, fontSize: 16, color: colors.snow },
  darkSub: { fontFamily: font.regular, fontSize: 13, color: colors.ash },
  darkHint: { fontFamily: font.regular, fontSize: 12, color: colors.fog, marginTop: 2 },
  note: { backgroundColor: colors.paper, borderRadius: 16, padding: 12, gap: 2, marginTop: 4 },
  noteLabel: { fontFamily: font.medium, fontSize: 12, color: colors.fog },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 6, paddingTop: 8 },
});
