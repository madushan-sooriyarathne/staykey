import { useLocalSearchParams } from "expo-router";
import { Text, View } from "react-native";
import { InfoNote, OptionRow, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, Hint, KV, SectionHeader, ui } from "@/components/kit";
import { CHANNEL_LABEL } from "@/data/labels";
import { SettingsPage, useSettings } from "@/features/property/settings";

const KEYS = ["booking", "ical", "currency"] as const;
const SHOW = ["USD", "EUR", "GBP", "AUD", "INR", "LKR"];

/** Instant book or request to book, the reply window, and the currencies guests see. */
export default function BookingSettings() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  if (!draft || !property)
    return (
      <SettingsPage title="Booking settings" missing>
        {null}
      </SettingsPage>
    );
  const b = draft.booking;
  const setB = (patch: Partial<typeof b>) => set({ booking: { ...b, ...patch } });
  const unsynced = draft.ical.filter((f) => f.status !== "ok").map((f) => CHANNEL_LABEL[f.channel]);

  return (
    <SettingsPage title="Booking settings" dirty={dirty} onSave={save} status={status}>
      <SectionHeader title="How guests book" />
      <OptionRow
        title="Instant book"
        description="Dates are confirmed as soon as the guest pays."
        selected={b.mode === "instant"}
        onPress={() => setB({ mode: "instant" })}
      />
      <OptionRow
        title="Request to book"
        description="You approve each booking before the guest pays."
        selected={b.mode === "request"}
        onPress={() => setB({ mode: "request" })}
      />
      {b.mode === "instant" && unsynced.length ? (
        <InfoNote icon={I.warning}>
          Your {unsynced.join(" and ")}{" "}
          {unsynced.length > 1 ? "calendars aren't" : "calendar isn't"} syncing yet, so instant book
          could double book. Fix iCal sync first.
        </InfoNote>
      ) : null}
      {b.mode === "request" ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Reply to requests within" />
          <View style={{ flexDirection: "row", gap: 8 }}>
            {[12, 24, 48].map((h) => (
              <Pill
                key={h}
                label={`${h} hours`}
                selected={b.replyHours === h}
                onPress={() => setB({ replyHours: h })}
              />
            ))}
          </View>
          <Hint>Requests are declined automatically after this.</Hint>
        </View>
      ) : null}
      <View style={{ gap: 8 }}>
        <SectionHeader title="Dates held at checkout" />
        <View style={{ flexDirection: "row", gap: 8 }}>
          {[10, 15, 30].map((m) => (
            <Pill
              key={m}
              label={`${m} min`}
              selected={b.holdMinutes === m}
              onPress={() => setB({ holdMinutes: m })}
            />
          ))}
        </View>
        <Hint>While a guest completes payment, nobody else can book those nights.</Hint>
      </View>
      <SectionHeader title="Currency" />
      <Card>
        <KV label="Charge guests in" value={draft.currency} />
        <Text style={[ui.faint, { paddingTop: 8, paddingBottom: 8 }]}>Show prices in</Text>
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {SHOW.map((c) => (
            <Pill
              key={c}
              label={c}
              selected={b.displayCurrencies.includes(c)}
              onPress={() =>
                c !== draft.currency &&
                setB({
                  displayCurrencies: b.displayCurrencies.includes(c)
                    ? b.displayCurrencies.filter((x) => x !== c)
                    : [...b.displayCurrencies, c],
                })
              }
            />
          ))}
        </View>
        <Hint>
          Guests always pay in {draft.currency}. Other currencies are shown as an estimate.
        </Hint>
      </Card>
    </SettingsPage>
  );
}
