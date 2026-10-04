import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";
import Animated, { FadeInDown, FadeOut } from "react-native-reanimated";
import { RangePicker } from "@/components/calendar";
import { Button, Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Hint, SectionHeader, SheetPage, ui } from "@/components/kit";
import { formatShort, nightsBetween } from "@/data/dates";
import { useProperty } from "@/data/hooks";
import { BLOCK_REASON, plural } from "@/data/labels";
import { conflicts } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Block } from "@/data/types";
import { haptics } from "@/lib/haptics";

/** Closes a range for one or more units, with a reason and a note for the team. */
export default function BlockDates() {
  const params = useLocalSearchParams<{
    propertyId: string;
    unitId: string;
    from: string;
    to: string;
  }>();
  const property = useProperty(params.propertyId);
  const bookings = useData((s) => s.bookings);
  const blocks = useData((s) => s.blocks);
  const addBlock = useData((s) => s.addBlock);
  const [from, setFrom] = useState<string | null>(params.from ?? null);
  const [to, setTo] = useState<string | null>(params.to ?? null);
  const [editing, setEditing] = useState(false);
  const [units, setUnits] = useState<string[]>(params.unitId ? [params.unitId] : []);
  const [reason, setReason] = useState<Block["reason"]>("maintenance");
  const [note, setNote] = useState("");

  if (!property) return null;
  const nights = from && to ? nightsBetween(from, to) : 0;
  const clash =
    from && to
      ? units.flatMap(
          (u) => conflicts(property, bookings, blocks, { unitId: u, from, to }).bookings,
        )
      : [];

  return (
    <SheetPage title="Block dates" subtitle={property.name}>
      <View style={{ gap: 6 }}>
        <SectionHeader
          title="Dates"
          action={{ label: editing ? "Done" : "Change", onPress: () => setEditing((v) => !v) }}
        />
        <Text style={ui.strong}>
          {from ? formatShort(from) : "Pick dates"}
          {to ? ` to ${formatShort(to)}` : ""}
        </Text>
        <Text style={ui.faint}>{plural(nights, "night")}</Text>
      </View>
      {editing ? (
        <Animated.View entering={FadeInDown.duration(220)} exiting={FadeOut.duration(120)}>
          <RangePicker
            start={from}
            end={to}
            onChange={(a, b) => {
              setFrom(a);
              setTo(b);
            }}
          />
        </Animated.View>
      ) : null}

      {property.units.length > 1 ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Units" />
          <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
            {property.units.map((u) => (
              <Pill
                key={u.id}
                label={u.name}
                selected={units.includes(u.id)}
                onPress={() =>
                  setUnits((x) => (x.includes(u.id) ? x.filter((y) => y !== u.id) : [...x, u.id]))
                }
              />
            ))}
          </View>
        </View>
      ) : null}

      <View style={{ gap: 8 }}>
        <SectionHeader title="Reason" />
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
          {(Object.keys(BLOCK_REASON) as Block["reason"][]).map((r) => (
            <Pill
              key={r}
              label={BLOCK_REASON[r]}
              selected={reason === r}
              onPress={() => setReason(r)}
            />
          ))}
        </View>
      </View>

      <Field
        label="Note for your team"
        value={note}
        onChangeText={setNote}
        placeholder="Pool resurfacing"
      />
      {clash.length ? (
        <InfoNote icon={I.warning}>
          {clash[0]?.guest.name} is booked on some of these nights. Move or cancel that stay before
          blocking.
        </InfoNote>
      ) : (
        <Hint>
          Blocked nights go into your iCal export, so Airbnb and Booking.com close them too.
        </Hint>
      )}
      <Button
        testID="block-confirm"
        title={`Block ${plural(nights, "night")}`}
        disabled={!from || !to || nights <= 0 || units.length === 0 || clash.length > 0}
        onPress={() => {
          if (!from || !to) return;
          haptics.success();
          addBlock({
            propertyId: property.id,
            unitIds: units,
            from,
            to,
            reason,
            note: note.trim() || undefined,
          });
          router.back();
        }}
      />
    </SheetPage>
  );
}
