import { colors } from "@staykey/tokens";
import { router } from "expo-router";
import { StyleSheet, Text, View } from "react-native";
import { Body, Button, Card, Chip, font, Heading, Label, Screen } from "@/components/ui";
import { useProperties } from "@/lib/api";

const today = new Intl.DateTimeFormat("en-GB", {
  weekday: "long",
  day: "numeric",
  month: "long",
}).format(new Date());

export default function TodayScreen() {
  const { state } = useProperties();

  return (
    <Screen eyebrow={today} title="Today">
      <View style={styles.stats}>
        <Stat value={0} label="Arrivals" />
        <Stat value={0} label="Departures" />
        <Stat value={0} label="In house" />
      </View>

      {state.status === "error" && (
        <Card>
          <Chip label="API offline" tone="ember" />
          <Body>{state.message}</Body>
        </Card>
      )}

      {state.status === "ready" && state.items.length === 0 && (
        <Card>
          <Heading>Set up your first property</Heading>
          <Body>Add a property to get your booking page and start taking direct bookings.</Body>
          <Button title="Add a property" onPress={() => router.push("/property/new")} />
        </Card>
      )}

      {state.status === "ready" && state.items.length > 0 && (
        <Card>
          <Heading>No bookings yet</Heading>
          <Body>Share your booking page on WhatsApp and Instagram to get your first one.</Body>
          <Label tone="faint">{state.items[0]?.bookingPageUrl}</Label>
        </Card>
      )}
    </Screen>
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
});
