import { colors } from "@staykey/tokens";
import { SymbolView } from "expo-symbols";
import { useMemo, useState } from "react";
import { ScrollView, View } from "react-native";
import Animated, { Easing, FadeIn, FadeInDown, FadeOut } from "react-native-reanimated";
import { Field, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import {
  Appear,
  EmptyState,
  IconButton,
  List,
  PropertySwitcher,
  Segmented,
} from "@/components/kit";
import { Screen } from "@/components/ui";
import { addDays, formatMonth, today } from "@/data/dates";
import { useBookings, useCan, useFilter, useProperties } from "@/data/hooks";
import { HOLDING, isOTA } from "@/data/pricing";
import type { Booking } from "@/data/types";
import { BookingListRow } from "@/features/bookings/rows";

type Tab = "upcoming" | "pending" | "past" | "cancelled";
type SourceFilter = "all" | "direct" | "ota" | "manual";

const ease = Easing.out(Easing.cubic);

export default function BookingsScreen() {
  const properties = useProperties();
  const all = useBookings();
  const filter = useFilter((s) => s.propertyId);
  const setFilter = useFilter((s) => s.set);
  const can = useCan();
  const [tab, setTab] = useState<Tab>("upcoming");
  const [query, setQuery] = useState("");
  const [showFilters, setShowFilters] = useState(false);
  const [source, setSource] = useState<SourceFilter>("all");
  const byId = useMemo(() => Object.fromEntries(properties.map((p) => [p.id, p])), [properties]);
  const day = today();
  const multi = properties.length > 1 && filter === "all";

  const scoped = all.filter((b) => {
    if (filter !== "all" && b.propertyId !== filter) return false;
    if (
      source === "direct" &&
      (isOTA(b) || ["whatsapp", "phone", "walkin", "other"].includes(b.source))
    )
      return false;
    if (source === "ota" && !isOTA(b)) return false;
    if (source === "manual" && !["whatsapp", "phone", "walkin", "other"].includes(b.source))
      return false;
    const q = query.trim().toLowerCase();
    if (
      q &&
      !`${b.guest.name} ${b.ref} ${b.guest.phone ?? ""} ${b.guest.email ?? ""}`
        .toLowerCase()
        .includes(q)
    )
      return false;
    return true;
  });

  const lists: Record<Tab, Booking[]> = {
    upcoming: scoped
      .filter((b) => HOLDING.includes(b.status) && b.checkOut >= day && b.status !== "checked_out")
      .sort((a, b) => a.checkIn.localeCompare(b.checkIn)),
    pending: scoped
      .filter((b) => b.status === "requested" || b.status === "awaiting_payment")
      .sort((a, b) => a.checkIn.localeCompare(b.checkIn)),
    past: scoped
      .filter((b) => (b.checkOut < day || b.status === "checked_out") && HOLDING.includes(b.status))
      .sort((a, b) => b.checkIn.localeCompare(a.checkIn)),
    cancelled: scoped
      .filter((b) => b.status === "cancelled" || b.status === "declined")
      .sort((a, b) => b.checkIn.localeCompare(a.checkIn)),
  };
  const items = lists[tab];
  const pendingCount = all.filter(
    (b) => (filter === "all" || b.propertyId === filter) && b.status === "requested",
  ).length;

  const groups = groupFor(tab, items, day);

  return (
    <Screen
      title="Bookings"
      right={
        <IconButton
          testID="bookings-filter"
          icon={I.filter}
          label="Filter"
          dot={source !== "all"}
          onPress={() => setShowFilters((v) => !v)}
        />
      }
    >
      <Field
        testID="bookings-search"
        placeholder="Search guest or reference"
        value={query}
        onChangeText={setQuery}
        autoCorrect={false}
        prefix={<SymbolView name={I.search} tintColor={colors.fog} size={16} />}
      />
      <PropertySwitcher properties={properties} value={filter} onChange={setFilter} allowAll />
      {showFilters ? (
        <Animated.View
          entering={FadeInDown.duration(220).easing(ease)}
          exiting={FadeOut.duration(120)}
        >
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: 8 }}
          >
            {(
              [
                ["all", "All sources"],
                ["direct", "Direct"],
                ["manual", "WhatsApp and phone"],
                ["ota", "Airbnb and Booking.com"],
              ] as const
            ).map(([id, label]) => (
              <Pill key={id} label={label} selected={source === id} onPress={() => setSource(id)} />
            ))}
          </ScrollView>
        </Animated.View>
      ) : null}
      <Segmented
        testID="bookings-tab"
        options={[
          { id: "upcoming", label: "Upcoming" },
          { id: "pending", label: "Pending", count: pendingCount },
          { id: "past", label: "Past" },
          { id: "cancelled", label: "Cancelled" },
        ]}
        value={tab}
        onChange={setTab}
      />

      <Animated.View
        key={`${tab}-${filter}-${source}`}
        entering={FadeIn.duration(220)}
        style={{ gap: 12 }}
      >
        {items.length === 0 ? (
          <EmptyState
            icon={query ? I.search : I.calendar}
            title={query ? "No matches" : EMPTY[tab].title}
            body={query ? "Try a guest name, phone number or booking reference." : EMPTY[tab].body}
          />
        ) : (
          groups.map((g, i) => (
            <Appear key={g.label} index={i}>
              <View style={{ gap: 6 }}>
                <List title={g.label}>
                  {g.items.map((b) => (
                    <BookingListRow
                      key={b.id}
                      booking={b}
                      property={byId[b.propertyId]}
                      withProperty={multi}
                      showMoney={can("prices")}
                    />
                  ))}
                </List>
              </View>
            </Appear>
          ))
        )}
      </Animated.View>
    </Screen>
  );
}

const EMPTY: Record<Tab, { title: string; body: string }> = {
  upcoming: {
    title: "No upcoming stays",
    body: "New bookings from your page, widget and WhatsApp land here.",
  },
  pending: { title: "Nothing waiting on you", body: "Requests and unpaid bookings show up here." },
  past: { title: "No past stays yet", body: "Stays move here after check-out." },
  cancelled: { title: "No cancellations", body: "Cancelled and declined bookings are kept here." },
};

function groupFor(tab: Tab, items: Booking[], day: string): { label: string; items: Booking[] }[] {
  const groups: { label: string; items: Booking[] }[] = [];
  const push = (label: string, b: Booking) => {
    const g = groups.find((x) => x.label === label);
    if (g) g.items.push(b);
    else groups.push({ label, items: [b] });
  };
  for (const b of items) {
    if (tab === "upcoming") {
      if (b.checkIn <= day) push("Today", b);
      else if (b.checkIn <= addDays(day, 7)) push("This week", b);
      else if (b.checkIn.slice(0, 7) === day.slice(0, 7)) push("Later this month", b);
      else push(formatMonth(b.checkIn), b);
    } else if (tab === "pending") {
      push(b.status === "requested" ? "Reply needed" : "Awaiting payment", b);
    } else {
      push(formatMonth(b.checkIn), b);
    }
  }
  return groups;
}
