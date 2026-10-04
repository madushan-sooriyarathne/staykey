import type { BookingType } from "@staykey/api-client";
import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { type ReactNode, useState } from "react";
import {
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  TextInput,
  View,
} from "react-native";
import { Body, Button, font, Label } from "@/components/ui";
import { api } from "@/lib/api";

/**
 * Minimal version of onboarding screens 5 and 8: name, booking type, location and price.
 * Creates the property through the API, which also reserves its booking page address.
 */
export default function NewPropertyScreen() {
  const [name, setName] = useState("");
  const [location, setLocation] = useState("");
  const [bookingType, setBookingType] = useState<BookingType>("entire");
  const [rate, setRate] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const rateMinor = Math.round(Number(rate) * 100);
  const ready = name.trim().length >= 2 && rateMinor > 0 && !saving;

  async function save() {
    setSaving(true);
    setError(null);
    try {
      const { data, error } = await api.POST("/v1/properties", {
        body: {
          name: name.trim(),
          location: location.trim() || undefined,
          bookingType,
          currency: "USD",
          baseRate: rateMinor,
        },
      });
      if (data) router.back();
      else setError(error?.message ?? "Could not create the property.");
    } catch {
      setError("Can't reach the StayKey API.");
    } finally {
      setSaving(false);
    }
  }

  return (
    <KeyboardAvoidingView
      style={styles.flex}
      behavior={Platform.OS === "ios" ? "padding" : undefined}
    >
      <ScrollView contentContainerStyle={styles.content} keyboardShouldPersistTaps="handled">
        <Field label="Property name">
          <TextInput
            style={styles.input}
            value={name}
            onChangeText={setName}
            placeholder="Kingfisher Villa"
            placeholderTextColor={colors.ash}
          />
        </Field>

        <Field label="How do guests book?">
          <View style={styles.row}>
            <Choice
              label="The whole place"
              selected={bookingType === "entire"}
              onPress={() => setBookingType("entire")}
            />
            <Choice
              label="Individual rooms"
              selected={bookingType === "rooms"}
              onPress={() => setBookingType("rooms")}
            />
          </View>
        </Field>

        <Field label="Location">
          <TextInput
            style={styles.input}
            value={location}
            onChangeText={setLocation}
            placeholder="Unawatuna, Galle"
            placeholderTextColor={colors.ash}
          />
        </Field>

        <Field label="Price per night in USD">
          <TextInput
            style={styles.input}
            value={rate}
            onChangeText={setRate}
            placeholder="180"
            placeholderTextColor={colors.ash}
            keyboardType="decimal-pad"
          />
        </Field>

        {error ? <Body style={{ color: colors.emberInk }}>{error}</Body> : null}

        <Button
          title={saving ? "Saving..." : "Create booking page"}
          disabled={!ready}
          onPress={save}
        />
      </ScrollView>
    </KeyboardAvoidingView>
  );
}

function Field({ label, children }: { label: string; children: ReactNode }) {
  return (
    <View style={styles.field}>
      <Label>{label}</Label>
      {children}
    </View>
  );
}

function Choice({
  label,
  selected,
  onPress,
}: {
  label: string;
  selected: boolean;
  onPress: () => void;
}) {
  return (
    <Pressable
      accessibilityRole="radio"
      accessibilityState={{ selected }}
      onPress={onPress}
      style={[styles.choice, selected && styles.choiceOn]}
    >
      <Text style={[styles.choiceText, selected && { color: colors.obsidian }]}>{label}</Text>
    </Pressable>
  );
}

const styles = StyleSheet.create({
  flex: { flex: 1, backgroundColor: colors.paper },
  content: { padding: 20, gap: 16 },
  field: { gap: 6 },
  input: {
    height: 48,
    backgroundColor: colors.snow,
    borderColor: colors.cloud,
    borderWidth: 1,
    borderRadius: radius.input,
    paddingHorizontal: 16,
    fontFamily: font.regular,
    fontSize: 15,
    color: colors.graphite,
  },
  row: { flexDirection: "row", gap: 8 },
  choice: {
    flex: 1,
    padding: 16,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  choiceOn: { borderColor: colors.obsidian, borderWidth: 1.5 },
  choiceText: { fontFamily: font.medium, fontSize: 15, color: colors.steel },
});
