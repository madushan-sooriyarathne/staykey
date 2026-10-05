import { colors } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";
import { useCalendar, useSetRateOverrides } from "@/api/calendar";
import { Button, Field, Stepper, TextLink } from "@/components/controls";
import { Card, Hint, money, SheetPage, SwitchRow } from "@/components/kit";
import { font } from "@/components/ui";
import { eachNight, formatShort } from "@/data/dates";
import { useProperty } from "@/data/hooks";
import { plural } from "@/data/labels";
import { minNightsFor, nightlyRate } from "@/data/pricing";
import { haptics } from "@/lib/haptics";

/** Sets the price and minimum stay for selected nights without opening the full rate settings. */
export default function EditRates() {
  const params = useLocalSearchParams<{
    propertyId: string;
    unitId: string;
    from: string;
    to: string;
  }>();
  const property = useProperty(params.propertyId);
  const { overrides } = useCalendar(params.propertyId);
  const setOverrides = useSetRateOverrides();
  const nights = params.from && params.to ? eachNight(params.from, params.to) : [];
  const current = property
    ? nights.map((n) => nightlyRate(property, params.unitId, n, overrides))
    : [];
  const low = Math.min(...current);
  const high = Math.max(...current);
  const [price, setPrice] = useState(current.length ? String(Math.round(high / 100)) : "");
  const [minStay, setMinStay] = useState(
    property && params.from ? minNightsFor(property, params.unitId, params.from, overrides) : 1,
  );
  const [closed, setClosed] = useState(!!overrides[params.unitId]?.[params.from]?.closedToArrival);

  if (!property) return null;
  const unit = property.units.find((u) => u.id === params.unitId);
  const cur = property.currency;
  const minor = Math.round(Number(price || 0) * 100);

  return (
    <SheetPage
      title="Edit rates"
      subtitle={`${unit?.name ?? property.name}, ${formatShort(params.from)} to ${formatShort(params.to)}`}
    >
      <Field
        testID="rates-price"
        label={`Nightly price in ${cur}`}
        value={price}
        onChangeText={(t) => setPrice(t.replace(/[^\d.]/g, ""))}
        keyboardType="decimal-pad"
        prefix={
          <Text style={{ fontFamily: font.medium, fontSize: 16, color: colors.steel }}>
            {cur === "USD" ? "$" : "LKR"}
          </Text>
        }
      />
      {current.length ? (
        <Hint>
          Current rates for these nights:{" "}
          {low === high ? money(low, cur) : `${money(low, cur)} to ${money(high, cur)}`}
        </Hint>
      ) : null}
      <Card>
        <Stepper label="Minimum stay" value={minStay} min={1} max={30} onChange={setMinStay} />
        <View style={{ height: 1, backgroundColor: colors.cloud, marginVertical: 4 }} />
        <SwitchRow
          title="Closed to arrival"
          subtitle="Guests can stay but not check in"
          value={closed}
          onChange={setClosed}
        />
      </Card>
      <Button
        testID="rates-apply"
        title={`Apply to ${plural(nights.length, "night")}`}
        disabled={minor <= 0 || nights.length === 0}
        onPress={() => {
          haptics.success();
          // The calendar shows the change at once and puts it back if the server refuses.
          setOverrides.mutate({
            propertyId: property.id,
            unitIds: [params.unitId],
            from: params.from,
            to: params.to,
            price: minor,
            minNights: minStay,
            closedToArrival: closed,
          });
          router.back();
        }}
      />
      <TextLink
        title="Go back to my usual rates"
        onPress={() => {
          haptics.select();
          setOverrides.mutate({
            propertyId: property.id,
            unitIds: [params.unitId],
            from: params.from,
            to: params.to,
          });
          router.back();
        }}
      />
    </SheetPage>
  );
}
