import { colors, radius } from "@staykey/tokens";
import * as ImagePicker from "expo-image-picker";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, {
  FadeInDown,
  FadeOut,
  LinearTransition,
  SlideInDown,
  SlideOutDown,
  ZoomIn,
  ZoomOut,
} from "react-native-reanimated";
import { messageFor } from "@/api/errors";
import { uploadImage } from "@/api/uploads";
import { Tag } from "@/components/brand";
import { Button, InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import { EmptyState, Hint, Page } from "@/components/kit";
import { useProperty } from "@/data/hooks";
import { plural } from "@/data/labels";
import type { PropertyConfig } from "@/data/types";
import { useLiveSave } from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

/** Upload, reorder and pick the cover shown on the booking page. Changes save as you go. */
export default function Photos() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const property = useProperty(id);
  const live = useLiveSave(id);
  const [uploading, setUploading] = useState(0);
  const [uploadError, setUploadError] = useState<string | null>(null);
  const [selected, setSelected] = useState<string | null>(null);
  if (!property) return null;
  const photos = property.photos;
  const error = uploadError ?? live.error;

  async function add() {
    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ["images"],
      allowsMultipleSelection: true,
      quality: 0.8,
    });
    if (result.canceled) return;
    setUploadError(null);
    setUploading(result.assets.length);
    const added: PropertyConfig["photos"] = [];
    try {
      for (const asset of result.assets) {
        const { key, url } = await uploadImage(asset.uri, "photo");
        added.push({ uri: url, key });
        setUploading((n) => n - 1);
      }
    } catch (e) {
      haptics.error();
      setUploadError(messageFor(e));
    }
    setUploading(0);
    if (added.length && (await live.apply({ photos: [...photos, ...added] }))) haptics.success();
  }

  function move(uri: string, to: number) {
    const from = photos.findIndex((p) => p.uri === uri);
    if (from < 0 || to < 0 || to >= photos.length) return;
    const next = [...photos];
    const [item] = next.splice(from, 1);
    if (item) next.splice(to, 0, item);
    haptics.select();
    live.apply({ photos: next });
  }

  const index = selected ? photos.findIndex((p) => p.uri === selected) : -1;

  return (
    <Page
      title="Photos"
      action={{
        label: uploading ? "Uploading" : "Add",
        onPress: add,
        disabled: uploading > 0,
        testID: "photos-add",
      }}
      footer={
        selected && index >= 0 ? (
          <Animated.View
            entering={SlideInDown.springify().damping(20)}
            exiting={SlideOutDown.duration(160)}
            style={s.bar}
          >
            <Text style={s.barTitle}>Photo {index + 1}</Text>
            <View style={s.barRow}>
              <View style={{ flex: 1 }}>
                <Button
                  compact
                  variant="ghost"
                  title="Make cover"
                  disabled={index === 0}
                  onPress={() => move(selected, 0)}
                />
              </View>
              <View style={{ flex: 1 }}>
                <Button
                  compact
                  variant="ghost"
                  title="Earlier"
                  disabled={index === 0}
                  onPress={() => move(selected, index - 1)}
                />
              </View>
              <View style={{ flex: 1 }}>
                <Button
                  compact
                  variant="ghost"
                  title="Later"
                  disabled={index === photos.length - 1}
                  onPress={() => move(selected, index + 1)}
                />
              </View>
            </View>
            <Animated.View entering={FadeInDown} exiting={FadeOut}>
              <Button
                compact
                variant="ghost"
                title="Remove photo"
                icon={I.trash}
                onPress={() => {
                  haptics.warning();
                  live.apply({ photos: photos.filter((x) => x.uri !== selected) });
                  setSelected(null);
                }}
              />
            </Animated.View>
          </Animated.View>
        ) : undefined
      }
    >
      {error ? <InfoNote icon={I.warning}>{error}</InfoNote> : null}
      {uploading ? (
        <Hint>
          Uploading {plural(uploading, "photo")}. Keep this screen open until they finish.
        </Hint>
      ) : null}
      {photos.length === 0 && !uploading ? (
        <EmptyState
          icon={I.photo}
          title="No photos yet"
          body="Wide shots of the pool, bedrooms and views work best. The first photo is your cover."
          action={<Button compact title="Add photos" icon={I.plus} onPress={add} />}
        />
      ) : (
        <>
          <Hint>
            {plural(photos.length, "photo")}. Tap a photo to make it the cover, move it or remove
            it.
          </Hint>
          <View style={s.grid}>
            {photos.map((p, i) => (
              <Animated.View
                key={p.uri}
                entering={ZoomIn.duration(220)}
                exiting={ZoomOut.duration(160)}
                layout={LinearTransition.springify().damping(20).stiffness(200)}
                style={s.cell}
              >
                <Pressable
                  onPress={() => {
                    haptics.select();
                    setSelected(selected === p.uri ? null : p.uri);
                  }}
                  style={[s.photo, selected === p.uri && s.selected]}
                >
                  <Image source={{ uri: p.uri }} style={StyleSheet.absoluteFill} />
                  {i === 0 ? <Tag tone="ink" label="Cover" style={{ margin: 8 }} /> : null}
                </Pressable>
              </Animated.View>
            ))}
          </View>
        </>
      )}
    </Page>
  );
}

const s = StyleSheet.create({
  grid: { flexDirection: "row", flexWrap: "wrap", gap: 10 },
  cell: { width: "48.5%" },
  photo: {
    aspectRatio: 1,
    borderRadius: radius.card - 10,
    backgroundColor: colors.mist,
    overflow: "hidden",
  },
  selected: { borderWidth: 3, borderColor: colors.ember },
  bar: {
    flex: 1,
    backgroundColor: colors.snow,
    borderRadius: 26,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 14,
    gap: 10,
  },
  barTitle: { fontFamily: "DMSans_600SemiBold", fontSize: 15, color: colors.obsidian },
  barRow: { flexDirection: "row", gap: 8 },
});
