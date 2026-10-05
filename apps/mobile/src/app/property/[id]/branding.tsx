import { colors, radius } from "@staykey/tokens";
import * as Clipboard from "expo-clipboard";
import * as ImagePicker from "expo-image-picker";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Image, Linking, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn, LinearTransition } from "react-native-reanimated";
import { messageFor } from "@/api/errors";
import { uploadImage } from "@/api/uploads";
import { Button } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, List, ListRow, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { BRAND_COLORS } from "@/data/defaults";
import { SettingsPage, useSettings } from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

const KEYS = ["branding", "photos", "name", "bookingPageUrl"] as const;

/** Logo, button colour and page address, with a live preview of what guests see. */
export default function Branding() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [uploading, setUploading] = useState(false);
  const [uploadError, setUploadError] = useState<string | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Booking page" missing>
        {null}
      </SettingsPage>
    );
  const color = draft.branding.color;
  const display = draft.bookingPageUrl.replace(/^https?:\/\//, "");

  return (
    <SettingsPage
      title="Booking page"
      dirty={dirty && !uploading}
      onSave={save}
      status={uploadError ? { saving: false, error: uploadError } : status}
      footer={
        <View style={{ flex: 1 }}>
          <Button
            variant="ghost"
            title="Open booking page"
            icon={I.globe}
            onPress={() => Linking.openURL(draft.bookingPageUrl)}
          />
        </View>
      }
    >
      <Animated.View layout={LinearTransition.duration(220)} style={s.preview}>
        <View style={s.cover}>
          {draft.photos[0] ? (
            <Image source={{ uri: draft.photos[0].uri }} style={StyleSheet.absoluteFill} />
          ) : (
            <Text style={ui.faint}>Cover photo</Text>
          )}
        </View>
        <View style={s.previewHead}>
          {draft.branding.logoUri ? (
            <Image source={{ uri: draft.branding.logoUri }} style={s.logo} />
          ) : (
            <Avatar name={draft.name} size={36} tone="dark" />
          )}
          <Text style={s.previewName}>{draft.name}</Text>
        </View>
        <View style={{ flexDirection: "row", gap: 8 }}>
          <View style={s.fakeInput}>
            <Text style={ui.faint}>Check-in</Text>
          </View>
          <View style={s.fakeInput}>
            <Text style={ui.faint}>Check-out</Text>
          </View>
        </View>
        <Animated.View
          key={color}
          entering={FadeIn.duration(220)}
          style={[s.fakeButton, { backgroundColor: color }]}
        >
          <Text style={s.fakeButtonText}>Check availability</Text>
        </Animated.View>
      </Animated.View>

      <List>
        <ListRow
          title="Logo"
          leading={
            draft.branding.logoUri ? (
              <Image source={{ uri: draft.branding.logoUri }} style={s.logo} />
            ) : (
              <Avatar name={draft.name} size={36} tone="dark" />
            )
          }
          value={uploading ? "Uploading" : draft.branding.logoUri ? "Replace" : "Add"}
          onPress={async () => {
            if (uploading) return;
            const result = await ImagePicker.launchImageLibraryAsync({
              mediaTypes: ["images"],
              allowsEditing: true,
              aspect: [1, 1],
              quality: 0.8,
            });
            const asset = !result.canceled ? result.assets[0] : undefined;
            if (!asset) return;
            setUploading(true);
            setUploadError(null);
            try {
              const { key, url } = await uploadImage(asset.uri, "logo");
              set({ branding: { ...draft.branding, logoUri: url, logoKey: key } });
            } catch (e) {
              haptics.error();
              setUploadError(messageFor(e));
            }
            setUploading(false);
          }}
        />
        <View style={s.colorRow}>
          <Text style={[ui.body, { flex: 1 }]}>Button colour</Text>
          <View style={{ flexDirection: "row", gap: 8 }}>
            {BRAND_COLORS.map((c) => (
              <Pressable
                key={c}
                accessibilityLabel={`Colour ${c}`}
                accessibilityState={{ selected: c === color }}
                onPress={() => {
                  haptics.select();
                  set({ branding: { ...draft.branding, color: c } });
                }}
                style={[s.swatch, { backgroundColor: c }, c === color && s.swatchOn]}
              />
            ))}
          </View>
        </View>
        <ListRow
          title="Page address"
          subtitle={display}
          value="Copy"
          chevron={false}
          onPress={async () => {
            await Clipboard.setStringAsync(draft.bookingPageUrl);
          }}
        />
      </List>
      <Text style={[ui.faint, { paddingHorizontal: 4 }]}>
        Changing your page address is coming soon. Old links will keep working when it does.
      </Text>
    </SettingsPage>
  );
}

const s = StyleSheet.create({
  preview: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 12,
    gap: 10,
  },
  cover: {
    height: 120,
    borderRadius: 18,
    backgroundColor: colors.mist,
    overflow: "hidden",
    justifyContent: "flex-end",
    padding: 12,
  },
  previewHead: { flexDirection: "row", alignItems: "center", gap: 10 },
  previewName: { fontFamily: font.semibold, fontSize: 17, color: colors.obsidian },
  logo: { width: 36, height: 36, borderRadius: 18 },
  fakeInput: {
    flex: 1,
    height: 44,
    borderRadius: radius.input,
    borderWidth: 1,
    borderColor: colors.cloud,
    justifyContent: "center",
    paddingHorizontal: 12,
  },
  fakeButton: {
    height: 46,
    borderRadius: radius.button,
    alignItems: "center",
    justifyContent: "center",
  },
  fakeButtonText: { fontFamily: font.medium, fontSize: 15, color: colors.snow },
  colorRow: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 14 },
  swatch: { width: 26, height: 26, borderRadius: 13 },
  swatchOn: {
    borderWidth: 3,
    borderColor: colors.snow,
    outlineWidth: 2,
    outlineColor: colors.obsidian,
    outlineStyle: "solid",
  },
});
