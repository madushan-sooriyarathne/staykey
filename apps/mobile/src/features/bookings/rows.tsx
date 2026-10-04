import { colors } from "@staykey/tokens";
import { router } from "expo-router";
import type { ReactNode } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import { Avatar, money, PaymentTag } from "@/components/kit";
import { font } from "@/components/ui";
import { fromISO } from "@/data/dates";
import { isOTA, SOURCE_LABEL } from "@/data/pricing";
import type { Booking, PropertyConfig } from "@/data/types";
import { haptics } from "@/lib/haptics";

/** "Kingfisher Villa" for a whole villa, "Ocean Room" for a room, plus the property when mixing. */
export function placeLabel(
  p: PropertyConfig | undefined,
  b: Booking,
  withProperty = false,
): string {
  if (!p) return "";
  const unit = p.units.find((u) => u.id === b.unitId);
  if (p.bookingType === "entire" || !unit) return p.name;
  return withProperty ? `${unit.name}, ${p.name}` : unit.name;
}

export function guestLabel(b: Booking): string {
  return isOTA(b) ? `${SOURCE_LABEL[b.source]} guest` : b.guest.name;
}

function open(b: Booking) {
  haptics.select();
  router.push({ pathname: "/booking/[id]", params: { id: b.id } });
}

/** Row with the guest's initials, used on Today. */
export function GuestRow({
  booking,
  property,
  subtitle,
  trailing,
  showMoney = true,
}: {
  booking: Booking;
  property?: PropertyConfig;
  subtitle: string;
  trailing?: ReactNode;
  showMoney?: boolean;
}) {
  return (
    <Pressable
      testID={`guest-${booking.ref}`}
      onPress={() => open(booking)}
      style={({ pressed }) => [s.row, pressed && { opacity: 0.6 }]}
    >
      <Avatar name={isOTA(booking) ? "" : booking.guest.name} />
      <View style={{ flex: 1, gap: 2 }}>
        <Text style={s.name} numberOfLines={1}>
          {guestLabel(booking)}
        </Text>
        <Text style={s.sub} numberOfLines={1}>
          {subtitle}
        </Text>
      </View>
      {trailing ??
        (showMoney && property ? (
          <PaymentTag booking={booking} currency={property.currency} />
        ) : null)}
    </Pressable>
  );
}

/** Row with a date column, used on the Bookings list. */
export function BookingListRow({
  booking,
  property,
  withProperty,
  showMoney = true,
}: {
  booking: Booking;
  property?: PropertyConfig;
  withProperty?: boolean;
  showMoney?: boolean;
}) {
  const d = fromISO(booking.checkIn);
  const nights = Math.round((fromISO(booking.checkOut).getTime() - d.getTime()) / 86_400_000);
  const manual = ["whatsapp", "phone", "walkin", "other"].includes(booking.source);
  const via = manual
    ? SOURCE_LABEL[booking.source]
    : `${nights} ${nights === 1 ? "night" : "nights"}`;
  return (
    <Pressable
      testID={`booking-${booking.ref}`}
      onPress={() => open(booking)}
      style={({ pressed }) => [s.row, pressed && { opacity: 0.6 }]}
    >
      <View style={s.date}>
        <Text style={s.dateMonth}>{d.toLocaleString("en-GB", { month: "short" })}</Text>
        <Text style={s.dateDay}>{d.getDate()}</Text>
      </View>
      <View style={{ flex: 1, gap: 2 }}>
        <Text style={s.name} numberOfLines={1}>
          {guestLabel(booking)}
        </Text>
        <Text style={s.sub} numberOfLines={1}>
          {placeLabel(property, booking, withProperty)}, {via}
        </Text>
      </View>
      <View style={s.end}>
        {showMoney && property && !isOTA(booking) ? (
          <Text style={s.amount}>{money(booking.total, property.currency)}</Text>
        ) : null}
        {property ? <PaymentTag booking={booking} currency={property.currency} /> : null}
      </View>
    </Pressable>
  );
}

const s = StyleSheet.create({
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12 },
  name: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  sub: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  date: {
    width: 44,
    height: 48,
    borderRadius: 14,
    backgroundColor: colors.paper,
    alignItems: "center",
    justifyContent: "center",
  },
  dateMonth: {
    fontFamily: font.medium,
    fontSize: 11,
    color: colors.fog,
    textTransform: "uppercase",
  },
  dateDay: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian, marginTop: -2 },
  end: { alignItems: "flex-end", gap: 4 },
  amount: { fontFamily: font.semibold, fontSize: 15, color: colors.obsidian },
});
