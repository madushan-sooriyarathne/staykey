import { colors, radius } from "@staykey/tokens";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Pressable, StyleSheet, Text, View } from "react-native";
import Animated, { LinearTransition } from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Field, InfoNote, Pill, Stepper } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Hint, money, Page, SectionHeader, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { uid } from "@/data/defaults";
import { useProperty } from "@/data/hooks";
import { plural } from "@/data/labels";
import { physicalUnits } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Unit } from "@/data/types";
import { DashedButton, InlineEditor, MoneyField, useLiveSave } from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

/** Bookable rooms or the whole villa. A linked unit blocks its rooms when booked, and the reverse. */
export default function Units() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const property = useProperty(id);
  const bookings = useData((s) => s.bookings);
  const live = useLiveSave(id);
  const [editing, setEditing] = useState<Unit | null>(null);
  if (!property) return null;
  const rooms = physicalUnits(property);

  async function save(u: Unit) {
    if (!property) return;
    const exists = property.units.some((x) => x.id === u.id);
    const saved = await live.apply({
      units: exists ? property.units.map((x) => (x.id === u.id ? u : x)) : [...property.units, u],
    });
    if (!saved) return;
    haptics.success();
    setEditing(null);
  }

  async function remove(u: Unit) {
    if (!property) return;
    haptics.warning();
    const saved = await live.apply({
      units: property.units
        .filter((x) => x.id !== u.id)
        .map((x) => ({ ...x, linkedUnitIds: x.linkedUnitIds?.filter((l) => l !== u.id) })),
    });
    if (saved) setEditing(null);
  }

  return (
    <Page
      title="Units"
      action={{ label: "Add", onPress: () => setEditing(blank()), testID: "units-add" }}
    >
      {live.error ? <InfoNote icon={I.warning}>{live.error}</InfoNote> : null}
      <Hint>
        {property.name},{" "}
        {property.bookingType === "entire" ? "booked as a whole" : plural(rooms.length, "room")}
      </Hint>
      {property.units.map((u, i) =>
        editing?.id === u.id ? (
          <UnitEditor
            key={u.id}
            unit={editing}
            others={property.units.filter((x) => x.id !== u.id && !x.linkedUnitIds?.length)}
            currency={property.currency}
            canRemove={
              property.units.length > 1 &&
              !bookings.some(
                (b) =>
                  b.unitId === u.id && !["cancelled", "declined", "checked_out"].includes(b.status),
              )
            }
            onCancel={() => setEditing(null)}
            onSave={save}
            onRemove={() => remove(u)}
          />
        ) : (
          <Appear key={u.id} index={i}>
            <Animated.View layout={LinearTransition.duration(200)}>
              <Pressable
                testID={`unit-${i}`}
                onPress={() => {
                  haptics.select();
                  setEditing(u);
                }}
                style={({ pressed }) => [s.card, pressed && { opacity: 0.7 }]}
              >
                <View style={s.head}>
                  <Text style={s.name}>{u.name}</Text>
                  <Text style={ui.amount}>
                    {money(u.rate, property.currency)}
                    <Text style={ui.faint}> / night</Text>
                  </Text>
                </View>
                <Text style={ui.faint}>
                  Sleeps {u.sleeps}
                  {u.beds && !u.beds.startsWith("Sleeps") ? `, ${u.beds}` : ""}
                  {u.weekendRate
                    ? `. ${money(u.weekendRate, property.currency)} on Fri and Sat`
                    : ""}
                </Text>
                {u.linkedUnitIds?.length ? (
                  <Tag
                    tone="dash"
                    label={`Linked to ${plural(u.linkedUnitIds.length, "room")}`}
                    style={{ marginTop: 8 }}
                  />
                ) : null}
              </Pressable>
            </Animated.View>
          </Appear>
        ),
      )}
      {editing && !property.units.some((u) => u.id === editing.id) ? (
        <UnitEditor
          unit={editing}
          others={property.units.filter((x) => !x.linkedUnitIds?.length)}
          currency={property.currency}
          canRemove={false}
          onCancel={() => setEditing(null)}
          onSave={save}
          onRemove={() => {}}
        />
      ) : (
        <DashedButton
          title={property.bookingType === "entire" ? "Add a room or unit" : "Add a unit"}
          onPress={() => setEditing(blank())}
        />
      )}
    </Page>
  );
}

function blank(): Unit {
  return { id: uid("unit"), name: "", sleeps: 2, beds: "", rate: 0 };
}

function UnitEditor({
  unit,
  others,
  currency,
  canRemove,
  onCancel,
  onSave,
  onRemove,
}: {
  unit: Unit;
  others: Unit[];
  currency: "USD" | "LKR";
  canRemove: boolean;
  onCancel: () => void;
  onSave: (u: Unit) => void;
  onRemove: () => void;
}) {
  const [u, setU] = useState(unit);
  const set = (patch: Partial<Unit>) => setU((x) => ({ ...x, ...patch }));
  return (
    <InlineEditor
      title={unit.name ? `Edit ${unit.name}` : "New unit"}
      onCancel={onCancel}
      onSave={() =>
        onSave({
          ...u,
          name: u.name.trim(),
          linkedUnitIds: u.linkedUnitIds?.length ? u.linkedUnitIds : undefined,
        })
      }
      onDelete={canRemove ? onRemove : undefined}
      saveDisabled={u.name.trim().length < 2 || u.rate <= 0}
    >
      <Field
        testID="unit-name"
        label="Name"
        value={u.name}
        onChangeText={(name) => set({ name })}
        placeholder="Ocean Room"
      />
      <Stepper
        label="Sleeps"
        value={u.sleeps}
        min={1}
        max={30}
        onChange={(sleeps) => set({ sleeps })}
      />
      <Field
        label="Beds"
        value={u.beds}
        onChangeText={(beds) => set({ beds })}
        placeholder="1 king bed"
      />
      <View style={{ flexDirection: "row", gap: 8 }}>
        <View style={{ flex: 1 }}>
          <MoneyField
            testID="unit-rate"
            label="Sun to Thu"
            value={u.rate}
            onChange={(rate) => set({ rate })}
            currency={currency}
          />
        </View>
        <View style={{ flex: 1 }}>
          <MoneyField
            label="Fri and Sat"
            value={u.weekendRate ?? 0}
            onChange={(v) => set({ weekendRate: v || undefined })}
            currency={currency}
            placeholder="Same"
          />
        </View>
      </View>
      {others.length > 1 ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Books these rooms together (optional)" />
          <View style={s.pills}>
            {others.map((o) => (
              <Pill
                key={o.id}
                label={o.name}
                selected={!!u.linkedUnitIds?.includes(o.id)}
                onPress={() =>
                  set({
                    linkedUnitIds: u.linkedUnitIds?.includes(o.id)
                      ? u.linkedUnitIds.filter((x) => x !== o.id)
                      : [...(u.linkedUnitIds ?? []), o.id],
                  })
                }
              />
            ))}
          </View>
        </View>
      ) : null}
    </InlineEditor>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 4,
  },
  head: { flexDirection: "row", alignItems: "center", justifyContent: "space-between", gap: 8 },
  name: { fontFamily: font.semibold, fontSize: 16, color: colors.obsidian, flex: 1 },
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
});
