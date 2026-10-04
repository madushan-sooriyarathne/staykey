import { formatMoney } from "@staykey/api-client";
import { colors } from "@staykey/tokens";
import { router, useFocusEffect } from "expo-router";
import { useCallback } from "react";
import { ActivityIndicator, Linking, Pressable, StyleSheet, Text, View } from "react-native";
import { Body, Button, Card, Chip, font, Label, Screen } from "@/components/ui";
import { useProperties } from "@/lib/api";

export default function PropertiesScreen() {
  const { state, reload } = useProperties();

  // Refresh when returning from the "New property" modal.
  useFocusEffect(
    useCallback(() => {
      reload();
    }, [reload]),
  );

  return (
    <Screen
      title="Properties"
      right={
        <Pressable onPress={() => router.push("/property/new")} style={styles.add}>
          <Text style={styles.addText}>Add</Text>
        </Pressable>
      }
    >
      {state.status === "loading" && <ActivityIndicator color={colors.obsidian} />}

      {state.status === "error" && (
        <Card>
          <Chip label="API offline" tone="ember" />
          <Body>{state.message}</Body>
        </Card>
      )}

      {state.status === "ready" && state.items.length === 0 && (
        <Card>
          <Body>No properties yet.</Body>
          <Button title="Add your first property" onPress={() => router.push("/property/new")} />
        </Card>
      )}

      {state.status === "ready" &&
        state.items.map((p) => (
          <Card key={p.id} style={{ padding: 0, overflow: "hidden" }}>
            <View style={styles.photo}>
              <Label>Cover photo</Label>
            </View>
            <View style={styles.cardBody}>
              <Text style={styles.name}>{p.name}</Text>
              {p.location ? <Label tone="faint">{p.location}</Label> : null}
              <View style={styles.chips}>
                <Chip label={p.bookingType === "entire" ? "Entire place" : "By room"} />
                <Chip label="Page live" tone="spark" />
              </View>
              <Body>From {formatMoney(p.baseRate, p.currency)} a night</Body>
              <Pressable onPress={() => Linking.openURL(p.bookingPageUrl)}>
                <Text style={styles.link}>{p.bookingPageUrl.replace(/^https?:\/\//, "")}</Text>
              </Pressable>
            </View>
          </Card>
        ))}
    </Screen>
  );
}

const styles = StyleSheet.create({
  add: {
    height: 38,
    paddingHorizontal: 14,
    borderRadius: 14,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    justifyContent: "center",
  },
  addText: { fontFamily: font.medium, color: colors.iron, fontSize: 14 },
  photo: { height: 140, backgroundColor: colors.mist, justifyContent: "flex-end", padding: 12 },
  cardBody: { padding: 20, gap: 8 },
  name: { fontFamily: font.semibold, fontSize: 20, color: colors.obsidian },
  chips: { flexDirection: "row", gap: 6 },
  link: { fontFamily: font.medium, color: colors.obsidian, textDecorationLine: "underline" },
});
