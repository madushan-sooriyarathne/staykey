import { colors } from "@staykey/tokens";
import * as ImagePicker from "expo-image-picker";
import { router } from "expo-router";
import { useState } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Button, Field, TextLink } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, Card, Page, Segmented, Two, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { useData } from "@/data/store";
import { useOnboarding } from "@/features/onboarding/store";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

/** Name, phone, email and language, plus sign out and in-app account deletion. */
export default function Profile() {
  const account = useData((s) => s.account);
  const updateAccount = useData((s) => s.updateAccount);
  const resetData = useData((s) => s.reset);
  const resetSession = useSession((s) => s.reset);
  const ownerName = useSession((s) => s.ownerName);
  const clearDraft = useOnboarding((s) => s.clear);
  const [name, setName] = useState(account.name || ownerName);
  const [mail, setMail] = useState(account.email);
  const [language, setLanguage] = useState(account.language);
  const [photo, setPhoto] = useState(account.photoUri);
  const [confirm, setConfirm] = useState(false);
  const dirty =
    name !== (account.name || ownerName) ||
    mail !== account.email ||
    language !== account.language ||
    photo !== account.photoUri;

  function signOut() {
    haptics.warning();
    clearDraft();
    resetData();
    resetSession();
  }

  return (
    <Page
      title="Profile and account"
      action={{
        label: "Save",
        disabled: !dirty || name.trim().length < 2,
        testID: "profile-save",
        onPress: () => {
          haptics.success();
          updateAccount({ name: name.trim(), email: mail.trim(), language, photoUri: photo });
          router.back();
        },
      }}
    >
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
      <Field label="Name" value={name} onChangeText={setName} autoCapitalize="words" />
      <Field
        label="Phone"
        value={account.phone || "+94 77 123 4567"}
        editable={false}
        suffix={<Tag tone="soft" label="Verified" />}
      />
      <Field
        label="Email"
        value={mail}
        onChangeText={setMail}
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
          onChange={setLanguage}
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
