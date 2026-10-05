import { router, useLocalSearchParams } from "expo-router";
import { View } from "react-native";
import { Pill, Stepper } from "@/components/controls";
import { Card, Hint, List, ListRow, SectionHeader } from "@/components/kit";
import { plural } from "@/data/labels";
import { SettingsPage, useSettings } from "@/features/property/settings";

const KEYS = ["rules", "seasons"] as const;
const DAYS = [
  { n: 1, label: "Mon" },
  { n: 2, label: "Tue" },
  { n: 3, label: "Wed" },
  { n: 4, label: "Thu" },
  { n: 5, label: "Fri" },
  { n: 6, label: "Sat" },
  { n: 0, label: "Sun" },
];
const NOTICE: { value: number | null; label: string }[] = [
  { value: 12, label: "Same day until 12:00 PM" },
  { value: 18, label: "Same day until 6:00 PM" },
  { value: null, label: "At least a day ahead" },
];

/** Minimum and maximum nights, advance notice, booking window and closed arrival days. */
export default function StayRules() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  if (!draft || !property)
    return (
      <SettingsPage title="Stay rules" missing>
        {null}
      </SettingsPage>
    );
  const r = draft.rules;
  const setRules = (patch: Partial<typeof r>) => set({ rules: { ...r, ...patch } });
  const seasonal = draft.seasons.filter((x) => x.minNights);

  return (
    <SettingsPage title="Stay rules" dirty={dirty} onSave={save} status={status}>
      <Card>
        <Stepper
          label="Minimum nights"
          value={r.minNights}
          min={1}
          max={30}
          onChange={(minNights) =>
            setRules({ minNights, maxNights: Math.max(minNights, r.maxNights) })
          }
        />
        <Stepper
          label="Maximum nights"
          value={r.maxNights}
          min={r.minNights}
          max={365}
          onChange={(maxNights) => setRules({ maxNights })}
        />
      </Card>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Advance notice" />
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {NOTICE.map((n) => (
            <Pill
              key={n.label}
              label={n.label}
              selected={r.sameDayCutoff === n.value}
              onPress={() => setRules({ sameDayCutoff: n.value })}
            />
          ))}
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Booking window" />
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {[3, 6, 12, 24].map((mo) => (
            <Pill
              key={mo}
              label={`${mo} months ahead`}
              selected={r.windowMonths === mo}
              onPress={() => setRules({ windowMonths: mo })}
            />
          ))}
        </View>
      </View>
      <View style={{ gap: 8 }}>
        <SectionHeader title="Closed arrival days" />
        <Hint>Guests cannot check in on the days you select.</Hint>
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {DAYS.map((d) => (
            <Pill
              key={d.n}
              label={d.label}
              selected={r.closedArrival.includes(d.n)}
              onPress={() =>
                setRules({
                  closedArrival: r.closedArrival.includes(d.n)
                    ? r.closedArrival.filter((x) => x !== d.n)
                    : [...r.closedArrival, d.n],
                })
              }
            />
          ))}
        </View>
      </View>
      <List title="Season rules">
        {seasonal.map((x) => (
          <ListRow
            key={x.id}
            title={x.name}
            subtitle={`Minimum ${plural(x.minNights ?? 1, "night")}`}
            onPress={() => router.push(`/property/${id}/rates` as never)}
          />
        ))}
        {seasonal.length === 0 ? (
          <ListRow
            title="No season rules"
            subtitle="Set a minimum stay on a season in Rates and seasons"
            onPress={() => router.push(`/property/${id}/rates` as never)}
          />
        ) : null}
      </List>
    </SettingsPage>
  );
}
