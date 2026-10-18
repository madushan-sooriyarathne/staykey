import { colors } from "@staykey/tokens";
import * as ImagePicker from "expo-image-picker";
import { router } from "expo-router";
import { useState } from "react";
import { Image, Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { useRenameAccount } from "@/api/accounts";
import { messageFor } from "@/api/errors";
import { useDeleteMe, useMe, useResendEmailConfirmation, useUpdateMe } from "@/api/me";
import { uploadAvatar } from "@/api/uploads";
import { Tag } from "@/components/brand";
import { Button, Field, InfoNote, TextLink } from "@/components/controls";
import { I } from "@/components/icons";
import { Avatar, Card, Page, Segmented, Two, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { useOnboarding } from "@/features/onboarding/store";
import { signOut as endSession } from "@/lib/auth";
import { shareAccountExport } from "@/lib/export";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

/**
 * Name, photo, phone, email and language from the API; for owners, the account's name and a copy
 * of its data; sign out and in-app deletion. A new email gets a confirmation link, and nothing
 * else is sent to it until it is confirmed.
 */
export default function Profile() {
  const me = useMe().data;
  const user = me?.user;
  const update = useUpdateMe();
  const rename = useRenameAccount();
  const deleteMe = useDeleteMe();
  const resend = useResendEmailConfirmation();
  // The address a confirmation link was just sent to, after saving a new email or resending.
  const [sentTo, setSentTo] = useState<string | null>(null);
  const resetSession = useSession((s) => s.reset);
  const ownerName = useSession((s) => s.ownerName);
  const accountId = useSession((s) => s.accountId);
  const clearDraft = useOnboarding((s) => s.clear);
  const account = me?.accounts.find((a) => a.id === accountId);
  const owner = account?.role === "owner";
  const saved = {
    name: user?.name || ownerName,
    email: user?.email ?? "",
    language: user?.language ?? "en",
    accountName: account?.name ?? "",
  };
  // Edits stay undefined until the person changes a field, so the form follows the server's copy.
  const [edits, setEdits] = useState<Partial<typeof saved>>({});
  const name = edits.name ?? saved.name;
  const mail = edits.email ?? saved.email;
  const language = edits.language ?? saved.language;
  const accountName = edits.accountName ?? saved.accountName;
  // A photo picked on this phone, waiting for Save; until then the saved one shows.
  const [picked, setPicked] = useState<string | null>(null);
  const photo = picked ?? user?.photoUrl ?? null;
  const [saving, setSaving] = useState(false);
  const [exporting, setExporting] = useState(false);
  const [confirm, setConfirm] = useState(false);
  const [typed, setTyped] = useState("");
  const [error, setError] = useState<string | null>(null);
  const dirty =
    name !== saved.name ||
    mail !== saved.email ||
    language !== saved.language ||
    (owner && accountName.trim() !== saved.accountName) ||
    picked !== null;

  async function save() {
    setError(null);
    setSaving(true);
    try {
      const photoKey = picked ? (await uploadAvatar(picked)).key : undefined;
      if (owner && accountName.trim() !== saved.accountName) {
        await rename.mutateAsync(accountName.trim());
      }
      const email = mail.trim();
      await update.mutateAsync({ name: name.trim(), email, language, photoKey });
      haptics.success();
      if (email && email !== saved.email) {
        // Stay, so the person sees where the confirmation link went.
        setEdits({});
        setPicked(null);
        setSentTo(email);
        return;
      }
      router.back();
    } catch (e) {
      haptics.error();
      setError(messageFor(e));
    } finally {
      setSaving(false);
    }
  }

  async function sendAgain() {
    setError(null);
    try {
      await resend.mutateAsync();
      setSentTo(saved.email);
    } catch (e) {
      setError(messageFor(e));
    }
  }

  async function download() {
    setError(null);
    setExporting(true);
    try {
      const r = await shareAccountExport();
      if (!r.ok) setError(r.message);
    } catch (e) {
      setError(messageFor(e));
    } finally {
      setExporting(false);
    }
  }

  async function remove() {
    setError(null);
    try {
      await deleteMe.mutateAsync();
      signOut();
    } catch (e) {
      haptics.error();
      setError(messageFor(e));
    }
  }

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
        disabled: !dirty || name.trim().length < 2 || (owner && !accountName.trim()) || saving,
        testID: "profile-save",
        onPress: save,
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
          if (!r.canceled && r.assets[0]) setPicked(r.assets[0].uri);
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
        suffix={
          saved.email && mail === saved.email ? (
            user?.emailConfirmed ? (
              <Tag tone="soft" label="Confirmed" />
            ) : (
              <Tag tone="ember" label="Not confirmed" />
            )
          ) : null
        }
        testID="profile-email"
      />
      {sentTo && sentTo === saved.email && !user?.emailConfirmed ? (
        <InfoNote icon={I.mail} testID="profile-email-sent">
          We sent a confirmation link to {sentTo}. Open it to start getting StayKey emails there.
        </InfoNote>
      ) : saved.email && mail === saved.email && user && !user.emailConfirmed ? (
        <View style={s.unconfirmed}>
          <Text style={[ui.muted, { flex: 1 }]}>Not confirmed yet. Check your inbox.</Text>
          <TextLink title="Send again" onPress={sendAgain} testID="profile-email-resend" />
        </View>
      ) : null}
      {owner ? (
        <Field
          label="Account name"
          value={accountName}
          onChangeText={(v) => setEdits((e) => ({ ...e, accountName: v }))}
          autoCapitalize="words"
          testID="profile-account-name"
        />
      ) : null}
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
      {owner ? (
        <Button
          variant="ghost"
          title="Download your data"
          loading={exporting}
          onPress={download}
          testID="profile-export"
        />
      ) : null}
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
              {owner
                ? "You're signed out everywhere and your booking page goes offline straight away. Your properties, bookings and team are deleted after 30 days; sign in again before then to keep them. Cancel any App Store or Google Play plan first."
                : "You're signed out everywhere and leave the teams you're on. Your profile is deleted after 30 days; sign in again before then to keep it."}
            </Text>
            <Field
              label="Type DELETE to confirm"
              value={typed}
              onChangeText={setTyped}
              autoCapitalize="characters"
              testID="profile-delete-confirm"
            />
            <View style={{ paddingTop: 12 }}>
              <Two>
                <Button
                  compact
                  variant="ghost"
                  title="Keep account"
                  onPress={() => {
                    setConfirm(false);
                    setTyped("");
                  }}
                />
                <Button
                  compact
                  title="Delete account"
                  disabled={typed.trim() !== "DELETE"}
                  loading={deleteMe.isPending}
                  onPress={remove}
                  testID="profile-delete"
                />
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
  unconfirmed: { flexDirection: "row", alignItems: "center", gap: 12, marginTop: -4 },
  label: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
});
