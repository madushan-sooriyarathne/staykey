import { formatMoney, suggestSlug } from "@staykey/api-client";
import { colors } from "@staykey/tokens";
import { SymbolView } from "expo-symbols";
import { useEffect } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn, FadeInDown, LinearTransition } from "react-native-reanimated";
import { Card, styles as c, Divider, Field, InfoNote, Pill } from "@/components/controls";
import { font } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { baseRateMinor, type Channel, listsOnOTAs, useOnboarding } from "../store";
import type { StepId, StepProps } from "../types";

const CHANNELS: { id: Channel; label: string }[] = [
  { id: "airbnb", label: "Airbnb" },
  { id: "booking", label: "Booking.com" },
  { id: "agoda", label: "Agoda" },
  { id: "expedia", label: "Expedia" },
  { id: "whatsapp", label: "WhatsApp and phone" },
  { id: "none", label: "Nowhere yet" },
];

export function ChannelsStep() {
  const { draft, update } = useOnboarding();

  function toggle(id: Channel) {
    if (id === "none") {
      update({ channels: draft.channels.includes("none") ? [] : ["none"] });
      return;
    }
    const without = draft.channels.filter((c) => c !== id && c !== "none");
    update({ channels: draft.channels.includes(id) ? without : [...without, id] });
  }

  const otas = CHANNELS.filter(
    (c) =>
      ["airbnb", "booking", "agoda", "expedia"].includes(c.id) && draft.channels.includes(c.id),
  )
    .map((c) => c.label)
    .join(" and ");

  return (
    <View style={{ gap: 16 }}>
      <View style={s.pills}>
        {CHANNELS.map((ch) => (
          <Pill
            key={ch.id}
            testID={`channel-${ch.id}`}
            label={ch.label}
            selected={draft.channels.includes(ch.id)}
            onPress={() => toggle(ch.id)}
          />
        ))}
      </View>
      {listsOnOTAs(draft) ? (
        <InfoNote>
          Until your {otas} {otas.includes(" and ") ? "calendars are" : "calendar is"} synced,
          guests send a request and you confirm the dates. Syncing is your first step after going
          live.
        </InfoNote>
      ) : null}
    </View>
  );
}

const BOOKING_DOMAIN = "staykey.direct";

export function ReviewStep({ goTo, error, clearError }: StepProps) {
  const { draft, update } = useOnboarding();
  const rate = baseRateMinor(draft);
  const cover = draft.photos[0];

  // Suggest the address from the property name until the owner edits it.
  useEffect(() => {
    if (!draft.slugEdited) update({ slug: suggestSlug(draft.propertyName) });
  }, [draft.slugEdited, draft.propertyName, update]);

  const rows: { label: string; value: string; step: StepId }[] = [
    {
      label: "Property",
      value:
        draft.bookingType === "entire"
          ? `Whole place, sleeps ${draft.guests}`
          : `${draft.rooms.length} room ${draft.rooms.length === 1 ? "type" : "types"}`,
      step: "basics",
    },
    {
      label: "Price",
      value:
        draft.bookingType === "rooms"
          ? `From ${formatMoney(rate, draft.currency)} a night`
          : `${formatMoney(rate, draft.currency)}${draft.weekendOn && draft.weekendRate ? `, ${formatMoney(Math.round(Number(draft.weekendRate) * 100), draft.currency)} on weekends` : ""}`,
      step: draft.bookingType === "rooms" ? "rooms" : "price",
    },
    {
      label: "Payments",
      value:
        [draft.bankOn && "Bank transfer", draft.payAtProperty && "pay at property"]
          .filter(Boolean)
          .join(", ") || "None",
      step: "payment",
    },
    {
      label: "Cancellation",
      value: draft.policy[0]?.toUpperCase() + draft.policy.slice(1),
      step: "policy",
    },
    {
      label: "Bookings",
      value: listsOnOTAs(draft) ? "Request to book until synced" : "Instant book",
      step: "channels",
    },
  ];

  return (
    <View style={{ gap: 14 }}>
      <Animated.View entering={FadeInDown.delay(80).duration(260)} style={s.preview}>
        <View style={s.cover}>
          {cover ? (
            <Image source={{ uri: cover.uri }} style={StyleSheet.absoluteFill} />
          ) : (
            <Text style={c.muted}>Cover photo</Text>
          )}
        </View>
        <View style={s.previewRow}>
          <Text style={c.choiceTitle} numberOfLines={1}>
            {draft.propertyName}
          </Text>
          {rate > 0 ? (
            <Text style={c.muted}>From {formatMoney(rate, draft.currency)} a night</Text>
          ) : null}
        </View>
      </Animated.View>

      <Field
        testID="slug-input"
        label="Your booking page address"
        autoCapitalize="none"
        autoCorrect={false}
        value={draft.slug}
        error={error}
        onChangeText={(t) => {
          clearError();
          update({
            slug: t
              .toLowerCase()
              .replace(/[^a-z0-9-]/g, "")
              .slice(0, 40),
            slugEdited: true,
          });
        }}
        suffix={<Text style={c.muted}>.{BOOKING_DOMAIN}</Text>}
      />

      <Animated.View layout={LinearTransition}>
        <Card>
          {rows.map((row, i) => (
            <View key={row.label}>
              {i > 0 ? <Divider /> : null}
              <Pressable
                onPress={() => {
                  haptics.back();
                  goTo(row.step);
                }}
                style={s.summaryRow}
              >
                <View style={{ flex: 1 }}>
                  <Text style={c.muted}>{row.label}</Text>
                  <Text style={c.body}>{row.value}</Text>
                </View>
                <Text style={s.edit}>Edit</Text>
              </Pressable>
            </View>
          ))}
        </Card>
      </Animated.View>

      {draft.photos.length === 0 ? (
        <Animated.View entering={FadeIn}>
          <View style={s.hintRow}>
            <SymbolView
              name={{ ios: "photo", android: "image", web: "image" }}
              tintColor={colors.fog}
              size={16}
            />
            <Text style={c.muted}>
              Add photos after publishing. They join your setup checklist.
            </Text>
          </View>
        </Animated.View>
      ) : null}
    </View>
  );
}

const s = StyleSheet.create({
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  preview: {
    padding: 10,
    gap: 10,
    borderRadius: 26,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.paper,
  },
  cover: {
    height: 120,
    borderRadius: 18,
    overflow: "hidden",
    backgroundColor: colors.mist,
    justifyContent: "flex-end",
    padding: 12,
  },
  previewRow: {
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    gap: 8,
    paddingHorizontal: 4,
    paddingBottom: 2,
  },
  summaryRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 12 },
  edit: { fontFamily: font.medium, fontSize: 14, color: colors.steel },
  hintRow: { flexDirection: "row", gap: 8, alignItems: "center" },
});
