import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { Image, StyleSheet, Text, View } from "react-native";
import { Tag } from "@/components/brand";
import { Button, PressScale } from "@/components/controls";
import { Appear } from "@/components/kit";
import { font, Screen } from "@/components/ui";
import { addMonths, formatMonthShort, monthStart, relativeDay, today } from "@/data/dates";
import { useProperties } from "@/data/hooks";
import { plural } from "@/data/labels";
import { nextArrival, occupancy, physicalUnits } from "@/data/pricing";
import { useData } from "@/data/store";
import type { PropertyConfig } from "@/data/types";
import { haptics } from "@/lib/haptics";

export default function PropertiesScreen() {
  const properties = useProperties();
  return (
    <Screen
      title="Properties"
      right={
        <Button
          compact
          variant="ghost"
          title="Add"
          icon={undefined}
          onPress={() => router.push("/property/new")}
          testID="properties-add"
        />
      }
    >
      {properties.map((p, i) => (
        <Appear key={p.id} index={i}>
          <PropertyCard property={p} />
        </Appear>
      ))}
    </Screen>
  );
}

function PropertyCard({ property: p }: { property: PropertyConfig }) {
  const bookings = useData((s) => s.bookings);
  const start = monthStart(today());
  const occ = occupancy(p, bookings, start, addMonths(start, 1));
  const next = nextArrival(p.id, bookings);
  const issues = p.ical.filter((f) => f.status === "error").length;
  const rooms = physicalUnits(p).length;
  const cover = p.photos[0];

  return (
    <PressScale
      testID={`property-${p.slug}`}
      scaleTo={0.985}
      onPress={() => {
        haptics.select();
        router.push({ pathname: "/property/[id]", params: { id: p.id } });
      }}
      style={s.card}
    >
      <View style={s.photo}>
        {cover ? (
          <Image source={{ uri: cover.uri }} style={StyleSheet.absoluteFill} />
        ) : (
          <Text style={s.photoText}>Add a cover photo</Text>
        )}
      </View>
      <View style={s.body}>
        <Text style={s.name}>{p.name}</Text>
        {p.location ? <Text style={s.sub}>{p.location}</Text> : null}
        <View style={s.chips}>
          <Tag
            tone="soft"
            label={p.bookingType === "entire" ? "Entire villa" : plural(rooms, "room")}
          />
          <Tag tone="spark" label="Page live" pulse />
          {issues ? <Tag tone="ember" label={plural(issues, "sync issue")} /> : null}
        </View>
        <View style={s.split}>
          <View style={{ flex: 1 }}>
            <Text style={s.statValue}>{Math.round(occ * 100)}%</Text>
            <Text style={s.sub}>{formatMonthShort(start)} occupancy</Text>
          </View>
          <View style={s.vr} />
          <View style={{ flex: 1 }}>
            <Text style={s.statValue}>{next ? relativeDay(next.checkIn) : "None"}</Text>
            <Text style={s.sub}>Next arrival</Text>
          </View>
        </View>
      </View>
    </PressScale>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    overflow: "hidden",
  },
  photo: { height: 150, backgroundColor: colors.mist, justifyContent: "flex-end", padding: 14 },
  photoText: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
  body: { padding: 16, gap: 6 },
  name: { fontFamily: font.semibold, fontSize: 20, color: colors.obsidian },
  sub: { fontFamily: font.regular, fontSize: 13, color: colors.fog },
  chips: { flexDirection: "row", flexWrap: "wrap", gap: 6, paddingTop: 4 },
  split: {
    flexDirection: "row",
    gap: 14,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
    marginTop: 10,
    paddingTop: 12,
  },
  vr: { width: 1, backgroundColor: colors.cloud },
  statValue: { fontFamily: font.semibold, fontSize: 20, color: colors.obsidian },
});
