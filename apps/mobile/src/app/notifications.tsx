import { colors, radius } from "@staykey/tokens";
import { StyleSheet, Text, View } from "react-native";
import { Appear, BrandSwitch, Hint, Page, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { NOTIFICATION_EVENTS } from "@/data/defaults";
import { useData } from "@/data/store";

/** Which events reach the owner by push and which by WhatsApp. */
export default function NotificationSettings() {
  const prefs = useData((s) => s.notifications);
  const set = useData((s) => s.setNotification);

  return (
    <Page title="Notifications">
      <Appear index={0} style={s.card}>
        <View style={s.head}>
          <View style={{ flex: 1 }} />
          <Text style={s.col}>Push</Text>
          <Text style={s.col}>WhatsApp</Text>
        </View>
        {NOTIFICATION_EVENTS.map((e) => (
          <View key={e.id} style={s.row}>
            <View style={{ flex: 1, gap: 2 }}>
              <Text style={ui.body}>{e.label}</Text>
              {e.hint ? <Text style={ui.faint}>{e.hint}</Text> : null}
            </View>
            <View style={s.cell}>
              <BrandSwitch
                label={`${e.label} by push`}
                value={prefs[e.id].push}
                onChange={(v) => set(e.id, "push", v)}
              />
            </View>
            <View style={s.cell}>
              <BrandSwitch
                label={`${e.label} on WhatsApp`}
                value={prefs[e.id].whatsapp}
                onChange={(v) => set(e.id, "whatsapp", v)}
              />
            </View>
          </View>
        ))}
      </Appear>
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
  col: { width: 72, textAlign: "center", fontFamily: font.medium, fontSize: 12, color: colors.fog },
  row: {
    flexDirection: "row",
    alignItems: "center",
    paddingVertical: 12,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
  },
  cell: { width: 72, alignItems: "center" },
});
