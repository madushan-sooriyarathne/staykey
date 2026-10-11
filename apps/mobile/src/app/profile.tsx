import { colors } from "@staykey/tokens";
import * as ImagePicker from "expo-image-picker";
import { router } from "expo-router";
import { useState } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { messageFor } from "@/api/errors";
import { useMe, useUpdateMe } from "@/api/me";
import { Tag } from "@/components/brand";
import { Button, Field, InfoNote, TextLink } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, Card, Page, Segmented, Two, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { useOnboarding } from "@/features/onboarding/store";
import { signOut as endSession } from "@/lib/auth";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

/**
 * Name, phone, email and language from the API, plus sign out and in-app account deletion. The
 * photo stays on this phone.
 */
export default function Profile() {
  const user = useMe().data?.user;
  const update = useUpdateMe();
  const resetSession = useSession((s) => s.reset);
  const ownerName = useSession((s) => s.ownerName);
  const savedPhoto = useSession((s) => s.photoUri);
  const setSavedPhoto = useSession((s) => s.setPhoto);
  const clearDraft = useOnboarding((s) => s.clear);
  const saved = {
    name: user?.name || ownerName,
    email: user?.email ?? "",
    language: user?.language ?? "en",
  };
  // Edits stay undefined until the person changes a field, so the form follows the server's copy.
  const [edits, setEdits] = useState<Partial<typeof saved>>({});
  const name = edits.name ?? saved.name;
  const mail = edits.email ?? saved.email;
  const language = edits.language ?? saved.language;
  const [photo, setPhoto] = useState(savedPhoto);
  const [confirm, setConfirm] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const dirty =
    name !== saved.name ||
    mail !== saved.email ||
    language !== saved.language ||
    photo !== savedPhoto;

  function signOut() {
    haptics.warning();
    endSession();
    clearDraft();
    resetSession();
  }

  return (
    <Page
      title="Profile and account"
      action={{
        label: "Save",
        disabled: !dirty || name.trim().length < 2 || update.isPending,
        testID: "profile-save",
        onPress: () => {
          setError(null);
          setSavedPhoto(photo);
          update.mutate(
            { name: name.trim(), email: mail.trim(), language },
            {
              onSuccess: () => {
                haptics.success();
                router.back();
              },
              onError: (e) => {
                haptics.error();
                setError(messageFor(e));
              },
            },
          );
        },
      }}
    >
      {error ? (
        <InfoNote icon={I.warning} testID="profile-error">
          {error}
        </InfoNote>
      ) : null}
      <Pressable
        style={s.head}
        onPress={async () => {
          const r = await ImagePicker.launchImageLibraryAsync({
            mediaTypes: ["images"],
            allowsEditing: true,
            aspect: [1, 1],
            quality: 0.7,
          });
          if (!r.canceled && r.assets[0]) setPhoto(r.assets[0].uri);
        }}
      >
        {photo ? (
          <Image source={{ uri: photo }} style={s.photo} />
        ) : (
          <Avatar name={name} size={80} tone="dark" />
        )}
        <Text style={ui.faint}>Change photo</Text>
      </Pressable>
      <Field
        label="Name"
        value={name}
        onChangeText={(v) => setEdits((e) => ({ ...e, name: v }))}
        autoCapitalize="words"
      />
      <Field
        label="Phone"
        value={user?.phone ?? ""}
        editable={false}
        suffix={<Tag tone="soft" label="Verified" />}
      />
      <Field
        label="Email"
        value={mail}
        onChangeText={(v) => setEdits((e) => ({ ...e, email: v }))}
        placeholder="you@example.com"
        keyboardType="email-address"
        autoCapitalize="none"
      />
      <View style={{ gap: 6 }}>
        <Text style={s.label}>App language</Text>
        <Segmented
          options={[
            { id: "en", label: "English" },
            { id: "si", label: "සිංහල" },
          ]}
          value={language}
          onChange={(v) => setEdits((e) => ({ ...e, language: v }))}
        />
      </View>
      <View style={{ paddingTop: 8 }}>
        <Button
          variant="ghost"
          title="Sign out"
          icon={I.logout}
          onPress={signOut}
          testID="profile-signout"
        />
      </View>
      {confirm ? (
        <Animated.View entering={FadeInDown.duration(220)} exiting={FadeOut.duration(120)}>
          <Card title="Delete your account?">
            <Text style={ui.muted}>
              Your booking page goes offline straight away, and your properties, bookings and team
              are removed after 30 days. Guests with upcoming stays keep their confirmation emails.
            </Text>
            <View style={{ paddingTop: 12 }}>
              <Two>
                <Button
                  compact
                  variant="ghost"
                  title="Keep account"
                  onPress={() => setConfirm(false)}
                />
                <Button compact title="Delete account" onPress={signOut} />
              </Two>
            </View>
          </Card>
        </Animated.View>
      ) : (
        <TextLink title="Delete account" onPress={() => setConfirm(true)} />
      )}
    </Page>
  );
}

const s = StyleSheet.create({
  head: { alignItems: "center", gap: 8, paddingVertical: 8 },
  photo: { width: 80, height: 80, borderRadius: 40 },
  label: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
});
