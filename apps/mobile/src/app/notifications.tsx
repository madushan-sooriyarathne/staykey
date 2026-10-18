import { colors, radius } from "@staykey/tokens";
import { StyleSheet, Text, View } from "react-native";
import { useMe } from "@/api/me";
import { useNotificationPrefs, useSetNotificationPref } from "@/api/messages";
import { Appear, BrandSwitch, Hint, Page, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { NOTIFICATION_EVENTS } from "@/data/defaults";

/**
 * Which events reach the signed-in person by push, WhatsApp and email, in this account. Email
 * waits until the person's address is confirmed.
 */
export default function NotificationSettings() {
  const prefs = useNotificationPrefs();
  const user = useMe().data?.user;
  const canEmail = Boolean(user?.email && user.emailConfirmed);
  const { mutate: save } = useSetNotificationPref();
  const pref = (event: string) => prefs.find((p) => p.event === event);

  return (
    <Page title="Notifications">
      <Appear index={0} style={s.card}>
        <View style={s.head}>
          <View style={{ flex: 1 }} />
          <Text style={s.col}>Push</Text>
          <Text style={s.col}>WhatsApp</Text>
          <Text style={s.col}>Email</Text>
        </View>
        {NOTIFICATION_EVENTS.map((e) => {
          const current = pref(e.id);
          if (!current) return null;
          return (
            <View key={e.id} style={s.row}>
              <View style={{ flex: 1, gap: 2 }}>
                <Text style={ui.body}>{e.label}</Text>
                {e.hint ? <Text style={ui.faint}>{e.hint}</Text> : null}
              </View>
              <View style={s.cell}>
                <BrandSwitch
                  label={`${e.label} by push`}
                  value={current.push}
                  onChange={(v) => save({ ...current, push: v })}
                />
              </View>
              <View style={s.cell}>
                <BrandSwitch
                  label={`${e.label} on WhatsApp`}
                  value={current.whatsapp}
                  onChange={(v) => save({ ...current, whatsapp: v })}
                />
              </View>
              <View style={s.cell}>
                <BrandSwitch
                  label={`${e.label} by email`}
                  value={canEmail && current.email}
                  disabled={!canEmail}
                  onChange={(v) => save({ ...current, email: v })}
                />
              </View>
            </View>
          );
        })}
      </Appear>
      {canEmail ? null : (
        <Hint>
          {user?.email
            ? "Confirm your email in Profile first to get these by email."
            : "Add and confirm your email in Profile first to get these by email."}
        </Hint>
      )}
      <Hint>Guest emails and messages are set in message templates.</Hint>
    </Page>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    paddingHorizontal: 16,
    paddingVertical: 6,
  },
  head: { flexDirection: "row", alignItems: "center", paddingVertical: 10 },
  col: { width: 64, textAlign: "center", fontFamily: font.medium, fontSize: 12, color: colors.fog },
  row: {
    flexDirection: "row",
    alignItems: "center",
    paddingVertical: 12,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
  },
  cell: { width: 64, alignItems: "center" },
});
