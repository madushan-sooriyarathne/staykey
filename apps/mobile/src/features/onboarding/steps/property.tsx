import { colors, radius } from "@staykey/tokens";
import * as ImagePicker from "expo-image-picker";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  LinearTransition,
  ZoomIn,
  ZoomOut,
} from "react-native-reanimated";
import {
  Button,
  Card,
  ChoiceCard,
  styles as c,
  Divider,
  Field,
  PressScale,
  SectionLabel,
  Stepper,
} from "@/components/controls";
import { Chip, font } from "@/components/ui";
import { haptics } from "@/lib/haptics";
import { type RoomType, useOnboarding } from "../store";

const layout = LinearTransition.springify().damping(24).stiffness(220);

export function BasicsStep() {
  const { draft, update } = useOnboarding();
  return (
    <View style={{ gap: 18 }}>
      <Field
        testID="property-name"
        label="Property name"
        autoFocus
        autoCapitalize="words"
        placeholder="Kingfisher Villa"
        value={draft.propertyName}
        onChangeText={(propertyName) => update({ propertyName })}
      />

      <View style={{ gap: 8 }}>
        <SectionLabel>How do guests book?</SectionLabel>
        <View style={{ flexDirection: "row", gap: 8 }}>
          <ChoiceCard
            testID="type-entire"
            title="The whole place"
            description="Guests book the entire villa"
            icon={{ ios: "house", android: "home", web: "home" }}
            selected={draft.bookingType === "entire"}
            onPress={() => update({ bookingType: "entire" })}
          />
          <ChoiceCard
            testID="type-rooms"
            title="Individual rooms"
            description="Guests book one or more rooms"
            icon={{ ios: "bed.double", android: "bed", web: "bed" }}
            selected={draft.bookingType === "rooms"}
            onPress={() => update({ bookingType: "rooms" })}
          />
        </View>
      </View>

      <Field
        label="Location"
        placeholder="Unawatuna, Galle"
        autoCapitalize="words"
        value={draft.location}
        onChangeText={(location) => update({ location })}
        prefix={
          <SymbolView
            name={{ ios: "mappin.and.ellipse", android: "location_on", web: "location_on" }}
            tintColor={colors.fog}
            size={18}
          />
        }
      />
    </View>
  );
}

export function SpaceStep() {
  const { draft, update } = useOnboarding();
  return (
    <Card>
      <Stepper
        label="Guests"
        hint="Most people who can stay"
        value={draft.guests}
        min={1}
        max={30}
        onChange={(guests) => update({ guests })}
      />
      <Divider />
      <Stepper
        label="Bedrooms"
        value={draft.bedrooms}
        max={20}
        onChange={(bedrooms) => update({ bedrooms })}
      />
      <Divider />
      <Stepper
        label="Beds"
        value={draft.beds}
        min={1}
        max={40}
        onChange={(beds) => update({ beds })}
      />
      <Divider />
      <Stepper
        label="Bathrooms"
        value={draft.bathrooms}
        max={20}
        onChange={(bathrooms) => update({ bathrooms })}
      />
    </Card>
  );
}

const emptyRoom = (): RoomType => ({
  id: Math.random().toString(36).slice(2),
  name: "",
  sleeps: 2,
  count: 1,
  rate: "",
});

export function RoomsStep() {
  const { draft, update } = useOnboarding();
  const [editing, setEditing] = useState<RoomType | null>(draft.rooms.length ? null : emptyRoom());
  const symbol = draft.currency === "LKR" ? "Rs " : "$";

  function save() {
    if (!editing) return;
    haptics.success();
    const exists = draft.rooms.some((r) => r.id === editing.id);
    update({
      rooms: exists
        ? draft.rooms.map((r) => (r.id === editing.id ? editing : r))
        : [...draft.rooms, editing],
    });
    setEditing(null);
  }

  const valid = editing && editing.name.trim().length >= 2 && Number(editing.rate) > 0;

  return (
    <View style={{ gap: 10 }}>
      {draft.rooms.map((room) => (
        <Animated.View
          key={room.id}
          entering={FadeIn}
          exiting={ZoomOut.duration(160)}
          layout={layout}
        >
          <Pressable
            onPress={() => {
              haptics.select();
              setEditing(room);
            }}
            style={s.roomCard}
          >
            <View style={{ flex: 1 }}>
              <Text style={c.choiceTitle}>{room.name}</Text>
              <Text style={c.muted}>
                Sleeps {room.sleeps}, {room.count} {room.count === 1 ? "room" : "rooms"}
              </Text>
            </View>
            <Text style={s.rate}>
              {symbol}
              {room.rate}
              <Text style={c.muted}> / night</Text>
            </Text>
            <Pressable
              accessibilityLabel={`Remove ${room.name}`}
              hitSlop={10}
              onPress={() => {
                haptics.select();
                update({ rooms: draft.rooms.filter((r) => r.id !== room.id) });
              }}
            >
              <SymbolView
                name={{ ios: "xmark", android: "close", web: "close" }}
                tintColor={colors.ash}
                size={14}
              />
            </Pressable>
          </Pressable>
        </Animated.View>
      ))}

      {editing ? (
        <Animated.View
          entering={FadeIn.duration(200)}
          exiting={FadeOut.duration(140)}
          layout={layout}
          style={s.editor}
        >
          <Field
            label="Room type"
            placeholder="Garden Room"
            autoCapitalize="words"
            autoFocus
            value={editing.name}
            onChangeText={(name) => setEditing({ ...editing, name })}
          />
          <Stepper
            label="Sleeps"
            value={editing.sleeps}
            min={1}
            max={12}
            onChange={(sleeps) => setEditing({ ...editing, sleeps })}
          />
          <Stepper
            label="Identical rooms"
            value={editing.count}
            min={1}
            max={30}
            onChange={(count) => setEditing({ ...editing, count })}
          />
          <Field
            label={`Price per night (${draft.currency})`}
            keyboardType="decimal-pad"
            placeholder="90"
            value={editing.rate}
            onChangeText={(rate) => setEditing({ ...editing, rate: rate.replace(/[^\d.]/g, "") })}
          />
          <View style={{ flexDirection: "row", gap: 8 }}>
            {draft.rooms.length ? (
              <View style={{ flex: 1 }}>
                <Button title="Cancel" variant="ghost" onPress={() => setEditing(null)} />
              </View>
            ) : null}
            <View style={{ flex: 1 }}>
              <Button title="Save room" disabled={!valid} onPress={save} />
            </View>
          </View>
        </Animated.View>
      ) : (
        <Animated.View entering={FadeIn.duration(180)} layout={layout}>
          <PressScale
            accessibilityRole="button"
            onPress={() => {
              haptics.select();
              setEditing(emptyRoom());
            }}
            style={s.addRoom}
          >
            <SymbolView
              name={{ ios: "plus", android: "add", web: "add" }}
              tintColor={colors.iron}
              size={16}
            />
            <Text style={[c.choiceTitle, { color: colors.iron }]}>Add a room type</Text>
          </PressScale>
        </Animated.View>
      )}
    </View>
  );
}

const MIN_PHOTOS = 3;

export function PhotosStep() {
  const { draft, update } = useOnboarding();

  async function pick() {
    haptics.select();
    const result = await ImagePicker.launchImageLibraryAsync({
      mediaTypes: ["images"],
      allowsMultipleSelection: true,
      selectionLimit: 20,
      quality: 0.85,
    });
    if (result.canceled) return;
    const added = result.assets.map((a) => ({ uri: a.uri, width: a.width, height: a.height }));
    update({ photos: [...draft.photos, ...added].slice(0, 30), photosLater: false });
  }

  function makeCover(index: number) {
    if (index === 0) return;
    haptics.select();
    const photos = [...draft.photos];
    const [chosen] = photos.splice(index, 1);
    if (chosen) update({ photos: [chosen, ...photos] });
  }

  function remove(index: number) {
    haptics.select();
    update({ photos: draft.photos.filter((_, i) => i !== index) });
  }

  return (
    <View style={{ gap: 12 }}>
      <View style={s.grid}>
        {draft.photos.map((photo, i) => (
          <Animated.View
            key={photo.uri}
            entering={ZoomIn.springify().damping(20)}
            exiting={ZoomOut.duration(150)}
            layout={layout}
            style={s.tileWrap}
          >
            <Pressable
              onPress={() => makeCover(i)}
              style={s.tile}
              accessibilityLabel={i === 0 ? "Cover photo" : "Make this the cover photo"}
            >
              <Image source={{ uri: photo.uri }} style={StyleSheet.absoluteFill} />
              {i === 0 ? (
                <View style={s.cover}>
                  <Chip label="Cover" />
                </View>
              ) : null}
              <Pressable
                hitSlop={8}
                onPress={() => remove(i)}
                style={s.remove}
                accessibilityLabel="Remove photo"
              >
                <SymbolView
                  name={{ ios: "xmark", android: "close", web: "close" }}
                  tintColor={colors.snow}
                  size={11}
                />
              </Pressable>
            </Pressable>
          </Animated.View>
        ))}
        <Animated.View layout={layout} style={s.tileWrap}>
          <PressScale
            testID="add-photos"
            accessibilityRole="button"
            onPress={pick}
            style={[s.tile, s.addTile]}
          >
            <SymbolView
              name={{ ios: "plus", android: "add", web: "add" }}
              tintColor={colors.iron}
              size={20}
            />
            <Text style={[c.muted, { color: colors.iron }]}>Add</Text>
          </PressScale>
        </Animated.View>
      </View>
      <Text style={c.muted}>
        {draft.photos.length >= MIN_PHOTOS
          ? "Tap a photo to make it the cover."
          : `${draft.photos.length} of ${MIN_PHOTOS} added. Wide shots of the pool, bedrooms and views work best.`}
      </Text>
    </View>
  );
}

const s = StyleSheet.create({
  roomCard: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    padding: 16,
    borderRadius: 24,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  rate: { fontFamily: font.semibold, fontSize: 15, color: colors.obsidian },
  editor: {
    gap: 10,
    padding: 16,
    borderRadius: 24,
    borderWidth: 1.5,
    borderColor: colors.obsidian,
    backgroundColor: colors.snow,
  },
  addRoom: {
    height: 52,
    flexDirection: "row",
    gap: 8,
    alignItems: "center",
    justifyContent: "center",
    borderRadius: radius.button,
    borderWidth: 1,
    borderStyle: "dashed",
    borderColor: colors.ash,
  },
  grid: { flexDirection: "row", flexWrap: "wrap", marginHorizontal: -4 },
  tileWrap: { width: "33.333%", padding: 4 },
  tile: {
    aspectRatio: 1,
    borderRadius: 18,
    overflow: "hidden",
    backgroundColor: colors.mist,
  },
  addTile: {
    alignItems: "center",
    justifyContent: "center",
    gap: 2,
    backgroundColor: "transparent",
    borderWidth: 1.5,
    borderStyle: "dashed",
    borderColor: colors.ash,
  },
  cover: { position: "absolute", left: 8, bottom: 8 },
  remove: {
    position: "absolute",
    top: 6,
    right: 6,
    width: 22,
    height: 22,
    borderRadius: 11,
    backgroundColor: "rgba(9,9,11,0.55)",
    alignItems: "center",
    justifyContent: "center",
  },
});
