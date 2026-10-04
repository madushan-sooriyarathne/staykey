import { suggestSlug } from "@staykey/api-client";
import { router } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { ApiError, messageFor } from "@/api/errors";
import { useCreateProperty } from "@/api/properties";
import { Button, ChoiceCard, Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Hint, Page, SectionHeader } from "@/components/kit";
import { localProperty } from "@/data/from-api";
import { useData } from "@/data/store";
import type { Currency } from "@/data/types";
import { MoneyField } from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

/** A second property in a minute: the basics, then everything else lives in its settings. */
export default function NewProperty() {
  const addProperty = useData((s) => s.addProperty);
  const [name, setName] = useState("");
  const [location, setLocation] = useState("");
  const [bookingType, setBookingType] = useState<"entire" | "rooms">("entire");
  const [currency, setCurrency] = useState<Currency>("USD");
  const [rate, setRate] = useState(0);
  const create = useCreateProperty();
  const [error, setError] = useState<string | null>(null);
  const saving = create.isPending;
  const ready = name.trim().length >= 2 && rate > 0 && !saving;

  async function save() {
    setError(null);
    const base = suggestSlug(name) || "property";
    // Try the name's address, then numbered ones if it's taken.
    for (let attempt = 0; attempt < 4; attempt++) {
      const slug = attempt === 0 ? base : `${base}-${attempt + 1}`;
      try {
        const created = await create.mutateAsync({
          name: name.trim(),
          slug,
          bookingType,
          location: location.trim() || undefined,
          currency,
          baseRate: rate,
        });
        const property = localProperty(created, { location: location.trim() });
        addProperty(property);
        haptics.success();
        router.replace({ pathname: "/property/[id]", params: { id: property.id } });
        return;
      } catch (e) {
        if (e instanceof ApiError && e.code === "slug_taken") continue;
        setError(messageFor(e));
        return;
      }
    }
    setError("We couldn't find a free booking page address. Try a different name.");
  }

  return (
    <Page
      title="Add a property"
      close
      footer={
        <View style={{ flex: 1 }}>
          <Button
            testID="new-property-save"
            title="Create booking page"
            icon={I.globe}
            disabled={!ready}
            loading={saving}
            onPress={save}
          />
        </View>
      }
    >
      <Field
        testID="new-property-name"
        label="Property name"
        value={name}
        onChangeText={setName}
        placeholder="Coral Bay House"
        autoCapitalize="words"
      />
      <View style={{ gap: 8 }}>
        <SectionHeader title="How do guests book?" />
        <View style={{ flexDirection: "row", gap: 8 }}>
          <ChoiceCard
            title="The whole place"
            icon={I.house}
            selected={bookingType === "entire"}
            onPress={() => setBookingType("entire")}
          />
          <ChoiceCard
            title="Individual rooms"
            icon={I.bed}
            selected={bookingType === "rooms"}
            onPress={() => setBookingType("rooms")}
          />
        </View>
      </View>
      <Field
        label="Location"
        value={location}
        onChangeText={setLocation}
        placeholder="Mirissa, Matara"
      />
      <View style={{ gap: 8 }}>
        <SectionHeader title="Currency" />
        <View style={{ flexDirection: "row", gap: 8 }}>
          {(["USD", "LKR"] as const).map((c) => (
            <Pill key={c} label={c} selected={currency === c} onPress={() => setCurrency(c)} />
          ))}
        </View>
      </View>
      <MoneyField
        testID="new-property-rate"
        key={currency}
        label={bookingType === "entire" ? "Price per night" : "Price per night for your first room"}
        value={rate}
        onChange={setRate}
        currency={currency}
      />
      <Hint>
        Photos, rooms, rates and payments can be set up from the property's settings next.
      </Hint>
      {error ? <InfoNote icon={I.wifi}>{error}</InfoNote> : null}
    </Page>
  );
}
