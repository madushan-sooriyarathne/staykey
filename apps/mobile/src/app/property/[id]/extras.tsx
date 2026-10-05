import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { IconBox } from "@/components/brand";
import { Field, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import {
  BrandSwitch,
  Hint,
  IconButton,
  List,
  ListRow,
  money,
  SectionHeader,
  SwitchRow,
} from "@/components/kit";
import { uid } from "@/data/defaults";
import { EXTRA_PER } from "@/data/labels";
import type { Extra } from "@/data/types";
import {
  DashedButton,
  InlineEditor,
  MoneyField,
  SettingsPage,
  useSettings,
} from "@/features/property/settings";

const KEYS = ["extras"] as const;

/** Add-ons such as transfers, breakfast and tours, priced per stay, night or guest. */
export default function Extras() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [editing, setEditing] = useState<Extra | null>(null);
  if (!draft || !property)
    return (
      <SettingsPage title="Extras" missing>
        {null}
      </SettingsPage>
    );
  const cur = property.currency;

  return (
    <SettingsPage title="Extras" dirty={dirty} onSave={save} status={status}>
      <Hint>Guests can add these during checkout.</Hint>
      {draft.extras.length ? (
        <List>
          {draft.extras.map((e) =>
            editing?.id === e.id ? null : (
              <ListRow
                key={e.id}
                leading={<IconBox name={I.gift} size={36} />}
                title={e.name}
                subtitle={`${money(e.price, cur)} ${EXTRA_PER[e.per]}${e.onRequest ? ", on request" : ""}`}
                chevron={false}
                trailing={
                  <View style={{ flexDirection: "row", alignItems: "center", gap: 10 }}>
                    <IconButton
                      icon={I.edit}
                      label={`Edit ${e.name}`}
                      onPress={() => setEditing(e)}
                    />
                    <BrandSwitch
                      label={e.name}
                      value={e.enabled}
                      onChange={(enabled) =>
                        set({
                          extras: draft.extras.map((x) => (x.id === e.id ? { ...x, enabled } : x)),
                        })
                      }
                    />
                  </View>
                }
              />
            ),
          )}
        </List>
      ) : null}
      {editing ? (
        <ExtraEditor
          extra={editing}
          currency={cur}
          isNew={!draft.extras.some((x) => x.id === editing.id)}
          onCancel={() => setEditing(null)}
          onSave={(e) => {
            set({
              extras: draft.extras.some((x) => x.id === e.id)
                ? draft.extras.map((x) => (x.id === e.id ? e : x))
                : [...draft.extras, e],
            });
            setEditing(null);
          }}
          onDelete={() => {
            set({ extras: draft.extras.filter((x) => x.id !== editing.id) });
            setEditing(null);
          }}
        />
      ) : (
        <DashedButton
          testID="extra-add"
          title="Add an extra"
          onPress={() =>
            setEditing({
              id: uid("ext"),
              name: "",
              price: 0,
              per: "stay",
              onRequest: false,
              enabled: true,
            })
          }
        />
      )}
    </SettingsPage>
  );
}

function ExtraEditor({
  extra,
  currency,
  isNew,
  onCancel,
  onSave,
  onDelete,
}: {
  extra: Extra;
  currency: "USD" | "LKR";
  isNew: boolean;
  onCancel: () => void;
  onSave: (e: Extra) => void;
  onDelete: () => void;
}) {
  const [e, setE] = useState(extra);
  return (
    <InlineEditor
      title={isNew ? "New extra" : `Edit ${extra.name}`}
      onCancel={onCancel}
      onSave={() => onSave({ ...e, name: e.name.trim() })}
      onDelete={isNew ? undefined : onDelete}
      saveDisabled={e.name.trim().length < 2 || e.price <= 0}
    >
      <Field
        testID="extra-name"
        label="Name"
        value={e.name}
        onChangeText={(name) => setE({ ...e, name })}
        placeholder="Airport transfer"
      />
      <MoneyField
        testID="extra-price"
        label="Price"
        value={e.price}
        currency={currency}
        onChange={(price) => setE({ ...e, price })}
      />
      <SectionHeader title="Charged" />
      <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
        {(Object.keys(EXTRA_PER) as Extra["per"][]).map((per) => (
          <Pill
            key={per}
            label={EXTRA_PER[per]}
            selected={e.per === per}
            onPress={() => setE({ ...e, per })}
          />
        ))}
      </View>
      <SwitchRow
        title="On request"
        subtitle="You confirm before it's added"
        value={e.onRequest}
        onChange={(onRequest) => setE({ ...e, onRequest })}
      />
    </InlineEditor>
  );
}
