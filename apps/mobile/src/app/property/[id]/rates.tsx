import { colors } from "@staykey/tokens";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { ScrollView, Text, View } from "react-native";
import { Field, Pill, Stepper } from "@/components/controls";
import { I } from "@/components/icons";
import {
  BrandSwitch,
  Card,
  IconButton,
  KV,
  List,
  ListRow,
  money,
  SectionHeader,
  ui,
} from "@/components/kit";
import { formatMonthDay } from "@/data/dates";
import { uid } from "@/data/defaults";
import { plural } from "@/data/labels";
import type { Season } from "@/data/types";
import {
  DashedButton,
  InlineEditor,
  MoneyField,
  SettingsPage,
  useSettings,
} from "@/features/property/settings";

const KEYS = ["units", "extraGuest", "seasons", "lengthDiscounts"] as const;
const MONTHS = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];

/** Weeknight and weekend rates, an extra-guest fee, seasons and longer-stay discounts. */
export default function RatesAndSeasons() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [season, setSeason] = useState<Season | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Rates and seasons" missing>
        {null}
      </SettingsPage>
    );
  const cur = property.currency;
  const m = (n: number) => money(n, cur);

  return (
    <SettingsPage title="Rates and seasons" dirty={dirty} onSave={save} status={status}>
      <Card title="Base rates" meta={cur}>
        {draft.units.map((u, i) => (
          <View key={u.id} style={{ gap: 8, paddingTop: i ? 12 : 0 }}>
            {draft.units.length > 1 ? <Text style={ui.strong}>{u.name}</Text> : null}
            <View style={{ flexDirection: "row", gap: 8 }}>
              <View style={{ flex: 1 }}>
                <MoneyField
                  testID={`rate-${i}`}
                  label="Sunday to Thursday"
                  value={u.rate}
                  currency={cur}
                  onChange={(rate) =>
                    set({ units: draft.units.map((x) => (x.id === u.id ? { ...x, rate } : x)) })
                  }
                />
              </View>
              <View style={{ flex: 1 }}>
                <MoneyField
                  label="Friday and Saturday"
                  value={u.weekendRate ?? 0}
                  placeholder="Same"
                  currency={cur}
                  onChange={(v) =>
                    set({
                      units: draft.units.map((x) =>
                        x.id === u.id ? { ...x, weekendRate: v || undefined } : x,
                      ),
                    })
                  }
                />
              </View>
            </View>
          </View>
        ))}
        <View style={{ flexDirection: "row", alignItems: "center", gap: 12, paddingTop: 14 }}>
          <View style={{ flex: 1 }}>
            <Text style={ui.body}>Charge for extra guests</Text>
            {draft.extraGuest ? (
              <Text style={ui.faint}>
                Each guest above {draft.extraGuest.above}, {m(draft.extraGuest.amount)} a night
              </Text>
            ) : null}
          </View>
          <BrandSwitch
            label="Charge for extra guests"
            value={!!draft.extraGuest}
            onChange={(on) =>
              set({
                extraGuest: on
                  ? {
                      above: Math.max(1, (draft.units[0]?.sleeps ?? 4) - 2),
                      amount: cur === "USD" ? 2500 : 750000,
                    }
                  : null,
              })
            }
          />
        </View>
        {draft.extraGuest ? (
          <View style={{ gap: 8, paddingTop: 8 }}>
            <Stepper
              label="Included guests"
              value={draft.extraGuest.above}
              min={1}
              max={30}
              onChange={(above) =>
                draft.extraGuest && set({ extraGuest: { ...draft.extraGuest, above } })
              }
            />
            <MoneyField
              label="Per extra guest, per night"
              value={draft.extraGuest.amount}
              currency={cur}
              onChange={(amount) =>
                draft.extraGuest && set({ extraGuest: { ...draft.extraGuest, amount } })
              }
            />
          </View>
        ) : null}
      </Card>

      <List title="Seasons" meta="Override base rates">
        {draft.seasons.map((x) =>
          season?.id === x.id ? null : (
            <ListRow
              key={x.id}
              title={x.name}
              subtitle={`${formatMonthDay(x.start)} to ${formatMonthDay(x.end)}${x.minNights ? `, ${plural(x.minNights, "night")} minimum` : ""}`}
              value={
                x.prices[draft.units[0]?.id ?? ""] != null
                  ? m(x.prices[draft.units[0]?.id ?? ""] as number)
                  : undefined
              }
              onPress={() => setSeason(x)}
            />
          ),
        )}
        {draft.seasons.length === 0 ? (
          <Text style={[ui.faint, { paddingVertical: 14 }]}>
            No seasons yet. Base rates apply all year.
          </Text>
        ) : null}
      </List>
      {season ? (
        <SeasonEditor
          season={season}
          units={draft.units}
          currency={cur}
          isNew={!draft.seasons.some((x) => x.id === season.id)}
          onCancel={() => setSeason(null)}
          onSave={(s) => {
            set({
              seasons: draft.seasons.some((x) => x.id === s.id)
                ? draft.seasons.map((x) => (x.id === s.id ? s : x))
                : [...draft.seasons, s],
            });
            setSeason(null);
          }}
          onDelete={() => {
            set({ seasons: draft.seasons.filter((x) => x.id !== season.id) });
            setSeason(null);
          }}
        />
      ) : (
        <DashedButton
          testID="season-add"
          title="Add a season"
          onPress={() =>
            setSeason({
              id: uid("sea"),
              name: "",
              start: "12-15",
              end: "01-15",
              prices: Object.fromEntries(draft.units.map((u) => [u.id, u.rate])),
            })
          }
        />
      )}

      <Card title="Longer stays">
        {draft.lengthDiscounts.map((d, i) => (
          <View
            key={`${d.nights}-${d.percent}`}
            style={{ flexDirection: "row", alignItems: "center", gap: 8 }}
          >
            <View style={{ flex: 1 }}>
              <KV label={`${d.nights} nights or more`} value={`${d.percent}% off`} />
            </View>
            <IconButton
              icon={I.trash}
              label="Remove discount"
              onPress={() =>
                set({ lengthDiscounts: draft.lengthDiscounts.filter((_, j) => j !== i) })
              }
            />
          </View>
        ))}
        <AddDiscount
          onAdd={(d) =>
            set({
              lengthDiscounts: [...draft.lengthDiscounts, d].sort((a, b) => a.nights - b.nights),
            })
          }
        />
      </Card>
    </SettingsPage>
  );
}

function AddDiscount({ onAdd }: { onAdd: (d: { nights: number; percent: number }) => void }) {
  const [open, setOpen] = useState(false);
  const [nights, setNights] = useState(7);
  const [percent, setPercent] = useState(10);
  if (!open) {
    return (
      <View style={{ paddingTop: 8 }}>
        <DashedButton title="Add a discount" onPress={() => setOpen(true)} />
      </View>
    );
  }
  return (
    <View style={{ gap: 8, paddingTop: 8 }}>
      <Stepper label="Nights or more" value={nights} min={2} max={90} onChange={setNights} />
      <Stepper label="Percent off" value={percent} min={1} max={60} onChange={setPercent} />
      <DashedButton
        title={`Add ${percent}% off ${nights}+ nights`}
        onPress={() => {
          onAdd({ nights, percent });
          setOpen(false);
        }}
      />
    </View>
  );
}

function MonthDay({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (v: string) => void;
}) {
  const [m = 1, d = 1] = value.split("-").map(Number);
  const days = new Date(2028, m, 0).getDate();
  const fmt = (mm: number, dd: number) =>
    `${String(mm).padStart(2, "0")}-${String(Math.min(dd, new Date(2028, mm, 0).getDate())).padStart(2, "0")}`;
  return (
    <View style={{ gap: 8 }}>
      <SectionHeader title={`${label}, ${formatMonthDay(value)}`} />
      <ScrollView
        horizontal
        showsHorizontalScrollIndicator={false}
        contentContainerStyle={{ gap: 6 }}
      >
        {MONTHS.map((name, i) => (
          <Pill
            key={name}
            label={name}
            selected={m === i + 1}
            onPress={() => onChange(fmt(i + 1, d))}
          />
        ))}
      </ScrollView>
      <Stepper label="Day" value={d} min={1} max={days} onChange={(dd) => onChange(fmt(m, dd))} />
    </View>
  );
}

function SeasonEditor({
  season,
  units,
  currency,
  isNew,
  onCancel,
  onSave,
  onDelete,
}: {
  season: Season;
  units: { id: string; name: string; rate: number }[];
  currency: "USD" | "LKR";
  isNew: boolean;
  onCancel: () => void;
  onSave: (s: Season) => void;
  onDelete: () => void;
}) {
  const [s, setS] = useState(season);
  return (
    <InlineEditor
      title={isNew ? "New season" : `Edit ${season.name}`}
      onCancel={onCancel}
      onSave={() => onSave({ ...s, name: s.name.trim(), minNights: s.minNights || undefined })}
      onDelete={isNew ? undefined : onDelete}
      saveDisabled={s.name.trim().length < 2}
    >
      <Field
        testID="season-name"
        label="Name"
        value={s.name}
        onChangeText={(name) => setS({ ...s, name })}
        placeholder="Peak season"
      />
      <MonthDay label="Starts" value={s.start} onChange={(start) => setS({ ...s, start })} />
      <MonthDay label="Ends" value={s.end} onChange={(end) => setS({ ...s, end })} />
      {units.map((u) => (
        <MoneyField
          key={u.id}
          label={units.length > 1 ? `${u.name}, per night` : "Price per night"}
          value={s.prices[u.id] ?? u.rate}
          currency={currency}
          onChange={(v) => setS({ ...s, prices: { ...s.prices, [u.id]: v } })}
        />
      ))}
      <Stepper
        label="Minimum nights"
        hint="0 uses your usual rule"
        value={s.minNights ?? 0}
        min={0}
        max={30}
        onChange={(minNights) => setS({ ...s, minNights })}
      />
      <Text style={[ui.faint, { color: colors.fog }]}>
        Seasons can run over the new year, like 15 Dec to 15 Jan.
      </Text>
    </InlineEditor>
  );
}
