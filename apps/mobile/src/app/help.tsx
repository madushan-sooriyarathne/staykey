import { colors, radius } from "@staykey/tokens";
import Constants from "expo-constants";
import { SymbolView } from "expo-symbols";
import { useState } from "react";
import { Pressable, StyleSheet, Text } from "react-native";
import Animated, { FadeInDown, FadeOut, LinearTransition } from "react-native-reanimated";
import { Field, PressScale } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Page, SectionHeader, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { email, openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

const SUPPORT_WHATSAPP = "+94 77 000 0000";

const TOPICS = [
  {
    q: "Connect your Airbnb calendar",
    a: "In Airbnb, open Calendar, then Availability, then Connect calendars, and copy the export link. Paste it into iCal sync on your property. Then copy your StayKey link from the same screen into Airbnb so it closes dates booked here.",
  },
  {
    q: "Add the widget to your website",
    a: "Open your property, then Share and embed. Copy the widget code and paste it where the booking box should appear, or tap Email developer to send it to whoever looks after your site.",
  },
  {
    q: "Record a WhatsApp booking",
    a: "Tap the plus on Today, pick the dates and fill in the guest. The price comes from your rates and you can change it. Turn on Deposit received if they've already paid something.",
  },
  {
    q: "Start taking card payments",
    a: "Open Payment methods and tap Connect card payments. PayHere emails you the documents they need, and approval usually takes a few days. Bank transfer and pay at property work in the meantime.",
  },
  {
    q: "Invite a caretaker",
    a: "Go to More, then Team, and tap Invite. Enter their WhatsApp number and pick Caretaker. They see arrivals, notes and guest contact, but never prices or payments.",
  },
];

/** Searchable help articles and a direct line to the support team. */
export default function Help() {
  const [query, setQuery] = useState("");
  const [open, setOpen] = useState<string | null>(null);
  const q = query.trim().toLowerCase();
  const topics = TOPICS.filter((t) => !q || `${t.q} ${t.a}`.toLowerCase().includes(q));

  return (
    <Page title="Help and support">
      <Field
        testID="help-search"
        placeholder="Search help articles"
        value={query}
        onChangeText={setQuery}
        prefix={<SymbolView name={I.search} tintColor={colors.fog} size={16} />}
      />
      <Appear index={0} style={{ flexDirection: "row", gap: 8 }}>
        <PressScale
          style={[s.contact, s.dark]}
          onPress={() => openWhatsApp(SUPPORT_WHATSAPP, "Hi StayKey, I need help with")}
        >
          <SymbolView name={I.chat} tintColor={colors.snow} size={20} />
          <Text style={[s.contactTitle, { color: colors.snow }]}>WhatsApp us</Text>
          <Text style={[ui.faint, { color: colors.ash }]}>Chat with our team</Text>
        </PressScale>
        <PressScale
          style={s.contact}
          onPress={() => email("support@staykey.direct", "Help with my account")}
        >
          <SymbolView name={I.mail} tintColor={colors.iron} size={20} />
          <Text style={s.contactTitle}>Email us</Text>
          <Text style={ui.faint}>For account issues</Text>
        </PressScale>
      </Appear>
      <SectionHeader
        title={
          q ? `${topics.length} ${topics.length === 1 ? "result" : "results"}` : "Popular topics"
        }
      />
      <Appear index={1} style={s.list}>
        {topics.map((t, i) => (
          <Animated.View
            key={t.q}
            layout={LinearTransition.duration(220)}
            style={i > 0 ? s.divided : undefined}
          >
            <Pressable
              onPress={() => {
                haptics.select();
                setOpen(open === t.q ? null : t.q);
              }}
              style={s.row}
            >
              <Text style={[ui.body, { flex: 1 }]}>{t.q}</Text>
              <SymbolView
                name={open === t.q ? I.chevronDown : I.chevronRight}
                tintColor={colors.ash}
                size={14}
              />
            </Pressable>
            {open === t.q ? (
              <Animated.Text
                entering={FadeInDown.duration(200)}
                exiting={FadeOut.duration(120)}
                style={[ui.muted, { paddingBottom: 14 }]}
              >
                {t.a}
              </Animated.Text>
            ) : null}
          </Animated.View>
        ))}
        {topics.length === 0 ? (
          <Text style={[ui.muted, { paddingVertical: 14 }]}>
            No articles match. Message us on WhatsApp and we'll help.
          </Text>
        ) : null}
      </Appear>
      <Text style={[ui.faint, { textAlign: "center" }]}>
        Version {Constants.expoConfig?.version ?? "1.0.0"}
      </Text>
    </Page>
  );
}

const s = StyleSheet.create({
  contact: {
    flex: 1,
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 4,
  },
  dark: { backgroundColor: colors.graphite, borderColor: colors.graphite },
  contactTitle: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian, marginTop: 6 },
  list: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    paddingHorizontal: 16,
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12, paddingVertical: 15 },
  divided: { borderTopWidth: 1, borderTopColor: colors.cloud },
});
