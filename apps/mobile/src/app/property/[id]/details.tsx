import { colors, radius } from "@staykey/tokens";
import { useLocalSearchParams } from "expo-router";
import { SymbolView } from "expo-symbols";
import { StyleSheet, Text, View } from "react-native";
import { Field, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { SectionHeader, Segmented, ui } from "@/components/kit";
import { AMENITIES } from "@/data/defaults";
import { clock } from "@/data/labels";
import { SettingsPage, TIMES_IN, TIMES_OUT, useSettings } from "@/features/property/settings";

const KEYS = [
  "name",
  "bookingType",
  "description",
  "location",
  "checkIn",
  "checkOut",
  "amenities",
] as const;

/** Name, booking type, description, location, times and amenities. */
export default function PropertyDetails() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  if (!draft || !property)
    return (
      <SettingsPage title="Details" missing>
        {null}
      </SettingsPage>
    );

  return (
    <SettingsPage
      title="Details"
      dirty={dirty && draft.name.trim().length > 1}
      onSave={save}
      status={status}
    >
      <Field
        testID="details-name"
        label="Property name"
        value={draft.name}
        onChangeText={(name) => set({ name })}
      />
      <View style={{ gap: 6 }}>
        <Text style={s.label}>Guests book</Text>
        <Segmented
          options={[
            { id: "entire", label: "The entire place" },
            { id: "rooms", label: "By room" },
          ]}
          value={draft.bookingType}
          onChange={(bookingType) => set({ bookingType })}
        />
      </View>
      <Field
        label="Description"
        value={draft.description}
        onChangeText={(description) => set({ description })}
        placeholder="What makes your place special"
        multiline
        style={{ minHeight: 96, textAlignVertical: "top" }}
      />
      <View style={{ gap: 6 }}>
        <Field
          label="Location"
          value={draft.location}
          onChangeText={(location) => set({ location })}
          placeholder="Unawatuna, Galle"
        />
        <View style={s.map}>
          <SymbolView name={I.pin} tintColor={colors.ember} size={26} />
          <Text style={ui.faint}>{draft.location || "Your map pin"}</Text>
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Check-in from" />
        <View style={s.pills}>
          {TIMES_IN.map((t) => (
            <Pill
              key={t}
              label={clock(t)}
              selected={draft.checkIn === t}
              onPress={() => set({ checkIn: t })}
            />
          ))}
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Check-out by" />
        <View style={s.pills}>
          {TIMES_OUT.map((t) => (
            <Pill
              key={t}
              label={clock(t)}
              selected={draft.checkOut === t}
              onPress={() => set({ checkOut: t })}
            />
          ))}
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Amenities" />
        <View style={s.pills}>
          {AMENITIES.map((a) => (
            <Pill
              key={a}
              label={a}
              selected={draft.amenities.includes(a)}
              onPress={() =>
                set({
                  amenities: draft.amenities.includes(a)
                    ? draft.amenities.filter((x) => x !== a)
                    : [...draft.amenities, a],
                })
              }
            />
          ))}
        </View>
      </View>
    </SettingsPage>
  );
}

const s = StyleSheet.create({
  label: { fontFamily: "DMSans_400Regular", fontSize: 13, color: colors.steel },
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  map: {
    height: 120,
    borderRadius: radius.card - 8,
    backgroundColor: colors.cloud,
    alignItems: "center",
    justifyContent: "center",
    gap: 6,
  },
});
