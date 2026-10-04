import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";
import { IconBox } from "@/components/brand";
import { Field, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import {
  BrandSwitch,
  Card,
  Hint,
  IconButton,
  KV,
  List,
  ListRow,
  money,
  Segmented,
} from "@/components/kit";
import { addDays, today } from "@/data/dates";
import { uid } from "@/data/defaults";
import { quote } from "@/data/pricing";
import type { Charge } from "@/data/types";
import {
  DashedButton,
  InlineEditor,
  MoneyField,
  SettingsPage,
  useSettings,
} from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

const KEYS = ["charges"] as const;

/** VAT, service charge and fees, with a preview of the guest's breakdown. */
export default function TaxesAndCharges() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, property } = useSettings(id, KEYS);
  const [editing, setEditing] = useState<Charge | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Taxes and charges" missing>
        {null}
      </SettingsPage>
    );
  const cur = property.currency;
  const unit = property.units[0];
  // Preview a 4-night weekday stay starting next Monday.
  const monday = addDays(today(), (8 - new Date().getDay()) % 7 || 7);
  const preview = unit
    ? quote(
        {
          ...property,
          charges: draft.charges,
          extras: [],
          promos: [],
          lengthDiscounts: [],
          extraGuest: null,
          seasons: [],
        },
        { unitId: unit.id, from: monday, to: addDays(monday, 4), adults: 2 },
      )
    : null;

  const describe = (c: Charge) =>
    c.kind === "percent"
      ? `${c.amount}% ${c.note ?? "on the room rate"}`
      : `${money(c.amount, cur)} per ${c.per}`;

  return (
    <SettingsPage title="Taxes and charges" dirty={dirty} onSave={save}>
      <List>
        {draft.charges.map((c) =>
          editing?.id === c.id ? null : (
            <ListRow
              key={c.id}
              leading={<IconBox name={c.kind === "percent" ? I.percent : I.cash} size={36} />}
              title={c.name}
              subtitle={describe(c)}
              chevron={false}
              trailing={
                <View style={{ flexDirection: "row", alignItems: "center", gap: 10 }}>
                  <IconButton
                    icon={I.edit}
                    label={`Edit ${c.name}`}
                    onPress={() => setEditing(c)}
                  />
                  <BrandSwitch
                    label={c.name}
                    value={c.enabled}
                    onChange={(enabled) =>
                      set({
                        charges: draft.charges.map((x) => (x.id === c.id ? { ...x, enabled } : x)),
                      })
                    }
                  />
                </View>
              }
            />
          ),
        )}
      </List>
      {editing ? (
        <ChargeEditor
          charge={editing}
          currency={cur}
          isNew={!draft.charges.some((x) => x.id === editing.id)}
          onCancel={() => setEditing(null)}
          onSave={(c) => {
            haptics.select();
            set({
              charges: draft.charges.some((x) => x.id === c.id)
                ? draft.charges.map((x) => (x.id === c.id ? c : x))
                : [...draft.charges, c],
            });
            setEditing(null);
          }}
          onDelete={() => {
            set({ charges: draft.charges.filter((x) => x.id !== editing.id) });
            setEditing(null);
          }}
        />
      ) : (
        <DashedButton
          title="Add a charge"
          onPress={() =>
            setEditing({
              id: uid("chg"),
              name: "",
              kind: "fixed",
              amount: 0,
              per: "stay",
              enabled: true,
            })
          }
        />
      )}
      {preview ? (
        <Card title="What guests see" meta="4-night stay">
          {preview.lines.map((l) => (
            <KV key={l.label} label={l.label} value={money(l.amount, cur)} />
          ))}
          <KV label="Total" value={money(preview.total, cur)} total />
        </Card>
      ) : null}
      <Hint>Charges are added on top of the nightly rate and shown line by line at checkout.</Hint>
    </SettingsPage>
  );
}

function ChargeEditor({
  charge,
  currency,
  isNew,
  onCancel,
  onSave,
  onDelete,
}: {
  charge: Charge;
  currency: "USD" | "LKR";
  isNew: boolean;
  onCancel: () => void;
  onSave: (c: Charge) => void;
  onDelete: () => void;
}) {
  const [c, setC] = useState(charge);
  return (
    <InlineEditor
      title={isNew ? "New charge" : `Edit ${charge.name}`}
      onCancel={onCancel}
      onSave={() => onSave({ ...c, name: c.name.trim() })}
      onDelete={isNew ? undefined : onDelete}
      saveDisabled={c.name.trim().length < 2 || c.amount <= 0}
    >
      <Field
        label="Name"
        value={c.name}
        onChangeText={(name) => setC({ ...c, name })}
        placeholder="Tourism levy"
      />
      <Segmented
        options={[
          { id: "percent", label: "Percentage" },
          { id: "fixed", label: "Fixed amount" },
        ]}
        value={c.kind}
        onChange={(kind) => setC({ ...c, kind, amount: 0 })}
      />
      {c.kind === "percent" ? (
        <Field
          label="Percent of the room rate"
          value={c.amount ? String(c.amount) : ""}
          onChangeText={(t) =>
            setC({ ...c, amount: Math.min(100, Number(t.replace(/\D/g, "")) || 0) })
          }
          keyboardType="number-pad"
          suffix={<Text>%</Text>}
        />
      ) : (
        <>
          <MoneyField
            label="Amount"
            value={c.amount}
            currency={currency}
            onChange={(amount) => setC({ ...c, amount })}
          />
          <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
            {(["stay", "night", "guest"] as const).map((per) => (
              <Pill
                key={per}
                label={`Per ${per}`}
                selected={c.per === per}
                onPress={() => setC({ ...c, per })}
              />
            ))}
          </View>
        </>
      )}
    </InlineEditor>
  );
}
