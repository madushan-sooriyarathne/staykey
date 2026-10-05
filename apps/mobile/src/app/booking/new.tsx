import { colors, radius } from "@staykey/tokens";
import * as Clipboard from "expo-clipboard";
import { router, useLocalSearchParams } from "expo-router";
import { useMemo, useState } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeInDown, FadeOut, LinearTransition } from "react-native-reanimated";
import { RangePicker } from "@/components/calendar";
import { Button, Field, InfoNote, Pill, Stepper } from "@/components/controls";
import { I } from "@/components/icons";
import { BrandSwitch, Card, Hint, KV, money, Page, SectionHeader, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { addDays, formatShort } from "@/data/dates";
import { useBooking, useFilter, useProperties } from "@/data/hooks";
import { METHOD_LABEL, plural } from "@/data/labels";
import { conflicts, depositFor, isOTA, minNightsFor, quote, SOURCE_LABEL } from "@/data/pricing";
import { useData } from "@/data/store";
import type { Payment, Source } from "@/data/types";
import { haptics } from "@/lib/haptics";

const MANUAL: { id: Source; label: string }[] = [
  { id: "whatsapp", label: "WhatsApp" },
  { id: "phone", label: "Phone" },
  { id: "walkin", label: "Walk-in" },
  { id: "other", label: "Other" },
];

const ease = Easing.out(Easing.cubic);

/** Manual entry for WhatsApp and phone bookings, and edits to any direct booking. */
export default function BookingForm() {
  const params = useLocalSearchParams<{
    id?: string;
    propertyId?: string;
    unitId?: string;
    from?: string;
    to?: string;
  }>();
  const existing = useBooking(params.id);
  const properties = useProperties();
  const bookings = useData((s) => s.bookings);
  const blocks = useData((s) => s.blocks);
  const overrides = useData((s) => s.overrides);
  const createBooking = useData((s) => s.createBooking);
  const updateBooking = useData((s) => s.updateBooking);
  const log = useData((s) => s.log);
  const filter = useFilter((s) => s.propertyId);

  const [propertyId, setPropertyId] = useState(
    existing?.propertyId ??
      params.propertyId ??
      (filter !== "all" ? filter : properties[0]?.id) ??
      "",
  );
  const property = properties.find((p) => p.id === propertyId) ?? properties[0];
  const [unitId, setUnitId] = useState(
    existing?.unitId ?? params.unitId ?? property?.units[0]?.id ?? "",
  );
  const unit = property?.units.find((u) => u.id === unitId) ?? property?.units[0];
  const [from, setFrom] = useState<string | null>(existing?.checkIn ?? params.from ?? null);
  const [to, setTo] = useState<string | null>(existing?.checkOut ?? params.to ?? null);
  const [picking, setPicking] = useState(!from || !to);
  const [adults, setAdults] = useState(existing?.adults ?? 2);
  const [children, setChildren] = useState(existing?.children ?? 0);
  const [name, setName] = useState(existing?.guest.name ?? "");
  const [phone, setPhone] = useState(existing?.guest.phone ?? "");
  const [mail, setMail] = useState(existing?.guest.email ?? "");
  const [source, setSource] = useState<Source>(existing?.source ?? "whatsapp");
  const [extras, setExtras] = useState<string[]>(existing?.extras ?? []);
  const [custom, setCustom] = useState(false);
  const [customTotal, setCustomTotal] = useState("");
  const [depositOn, setDepositOn] = useState(false);
  const [deposit, setDeposit] = useState("");
  const [method, setMethod] = useState<Payment["method"]>("bank");
  const [note, setNote] = useState(existing?.ownerNote ?? "");

  const q = useMemo(
    () =>
      property && unit && from && to
        ? quote(property, {
            unitId: unit.id,
            from,
            to,
            adults,
            children,
            extras,
            overrides: overrides[property.id],
          })
        : null,
    [property, unit, from, to, adults, children, extras, overrides],
  );
  const clash =
    property && unit && from && to
      ? conflicts(property, bookings, blocks, {
          unitId: unit.id,
          from,
          to,
          ignoreBookingId: existing?.id,
        })
      : null;
  const clashing = !!clash && (clash.bookings.length > 0 || clash.blocks.length > 0);
  const total = custom ? Math.round(Number(customTotal || 0) * 100) : (q?.total ?? 0);
  const minNights =
    property && unit && from ? minNightsFor(property, unit.id, from, overrides[property.id]) : 1;
  const ready =
    !!property && !!unit && !!from && !!to && name.trim().length > 1 && !clashing && total >= 0;

  if (!property || !unit) return null;
  const cur = property.currency;
  const symbol = cur === "USD" ? "$" : "LKR";
  const editing = !!existing;
  const fixedSource = editing && !MANUAL.some((m) => m.id === existing.source);

  const isBlocked = (night: string) => {
    if (!property) return false;
    return (
      conflicts(property, bookings, blocks, {
        unitId: unit.id,
        from: night,
        to: addDays(night, 1),
        ignoreBookingId: existing?.id,
      }).bookings.length > 0
    );
  };

  function save() {
    if (!ready || !from || !to || !property || !unit) return;
    const lines = custom
      ? [{ label: `${plural(q?.nights ?? 0, "night")}, agreed price`, amount: total }]
      : (q?.lines ?? []);
    const guest = {
      name: name.trim(),
      phone: phone.trim() || undefined,
      email: mail.trim() || undefined,
    };
    haptics.success();
    if (existing) {
      updateBooking(existing.id, {
        unitId: unit.id,
        checkIn: from,
        checkOut: to,
        adults,
        children,
        guest: { ...existing.guest, ...guest },
        extras,
        lines,
        total,
        ownerNote: note.trim() || undefined,
        source,
      });
      router.back();
      return;
    }
    const amount = Math.round(Number(deposit || 0) * 100);
    const created = createBooking(
      {
        propertyId: property.id,
        unitId: unit.id,
        source,
        status: "confirmed",
        guest,
        adults,
        children,
        checkIn: from,
        checkOut: to,
        lines,
        total,
        payments:
          depositOn && amount > 0
            ? [{ id: `pay_${Date.now()}`, amount, method, at: new Date().toISOString() }]
            : [],
        extras,
        ownerNote: note.trim() || undefined,
      },
      property,
    );
    log({
      kind: "booking",
      title: `You added ${created.guest.name}`,
      subtitle: `${unit.name}, ${SOURCE_LABEL[source]} booking`,
      bookingId: created.id,
      propertyId: property.id,
    });
    router.replace({ pathname: "/booking/[id]", params: { id: created.id } });
  }

  return (
    <Page
      title={editing ? "Edit booking" : "New booking"}
      close
      footer={
        <View style={{ flex: 1 }}>
          <Button
            testID="booking-save"
            title={editing ? "Save changes" : "Save booking"}
            disabled={!ready}
            onPress={save}
          />
        </View>
      }
    >
      {!editing && properties.length > 1 ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Property" />
          <ScrollView
            horizontal
            showsHorizontalScrollIndicator={false}
            contentContainerStyle={{ gap: 8 }}
          >
            {properties.map((p) => (
              <Pill
                key={p.id}
                label={p.name}
                selected={p.id === property.id}
                onPress={() => {
                  setPropertyId(p.id);
                  setUnitId(p.units[0]?.id ?? "");
                }}
              />
            ))}
          </ScrollView>
        </View>
      ) : null}

      {property.units.length > 1 ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Unit" />
          <View style={s.pills}>
            {property.units.map((u) => (
              <Pill
                key={u.id}
                label={u.name}
                selected={u.id === unit.id}
                onPress={() => setUnitId(u.id)}
              />
            ))}
          </View>
        </View>
      ) : (
        <Field label="Unit" value={unit.name} editable={false} />
      )}

      <View style={s.two}>
        <DateField
          label="Check-in"
          value={from}
          active={picking && !to}
          onPress={() => setPicking((v) => !v)}
        />
        <DateField
          label="Check-out"
          value={to}
          active={picking && !!from && !to}
          onPress={() => setPicking((v) => !v)}
        />
      </View>

      {picking ? (
        <Animated.View
          entering={FadeInDown.duration(240).easing(ease)}
          exiting={FadeOut.duration(120)}
          style={s.card}
        >
          <RangePicker
            start={from}
            end={to}
            isBlocked={isBlocked}
            allowPast={editing}
            onChange={(a, b) => {
              setFrom(a);
              setTo(b);
              if (a && b) setTimeout(() => setPicking(false), 250);
            }}
          />
        </Animated.View>
      ) : null}

      {clashing && clash ? (
        <InfoNote icon={I.warning}>
          {clash.bookings[0]
            ? `Those dates overlap ${isOTA(clash.bookings[0]) ? "an imported stay" : `${clash.bookings[0].guest.name}'s stay`}, ${formatShort(clash.bookings[0].checkIn)} to ${formatShort(clash.bookings[0].checkOut)}.`
            : "Some of those nights are blocked. Open them on the calendar first."}
        </InfoNote>
      ) : q && q.nights > 0 && q.nights < minNights ? (
        <Hint>
          Your minimum stay is {plural(minNights, "night")}. You can still save this booking.
        </Hint>
      ) : null}

      <Card>
        <Stepper label="Adults" value={adults} min={1} max={unit.sleeps} onChange={setAdults} />
        <Stepper
          label="Children"
          value={children}
          min={0}
          max={Math.max(0, unit.sleeps - adults)}
          onChange={setChildren}
        />
      </Card>

      <Field
        testID="guest-name"
        label="Guest name"
        value={name}
        onChangeText={setName}
        placeholder="Daniel Fernando"
        autoCapitalize="words"
      />
      <Field
        label="Phone"
        value={phone}
        onChangeText={setPhone}
        placeholder="+94 77 123 4567"
        keyboardType="phone-pad"
        suffix={
          <Pressable
            hitSlop={8}
            onPress={async () => {
              const text = await Clipboard.getStringAsync();
              if (text) {
                haptics.select();
                setPhone(text.trim().slice(0, 24));
              }
            }}
          >
            <Text style={s.paste}>Paste</Text>
          </Pressable>
        }
      />
      <Field
        label="Email (optional)"
        value={mail}
        onChangeText={setMail}
        placeholder="guest@example.com"
        keyboardType="email-address"
        autoCapitalize="none"
      />

      <View style={{ gap: 8 }}>
        <SectionHeader title="Booked through" />
        {fixedSource ? (
          <Text style={ui.muted}>{SOURCE_LABEL[existing.source]}</Text>
        ) : (
          <View style={s.pills}>
            {MANUAL.map((m) => (
              <Pill
                key={m.id}
                label={m.label}
                selected={source === m.id}
                onPress={() => setSource(m.id)}
              />
            ))}
          </View>
        )}
      </View>

      {property.extras.some((e) => e.enabled) ? (
        <View style={{ gap: 8 }}>
          <SectionHeader title="Extras" />
          <View style={s.pills}>
            {property.extras
              .filter((e) => e.enabled)
              .map((e) => (
                <Pill
                  key={e.id}
                  label={e.name}
                  selected={extras.includes(e.id)}
                  onPress={() =>
                    setExtras((x) =>
                      x.includes(e.id) ? x.filter((y) => y !== e.id) : [...x, e.id],
                    )
                  }
                />
              ))}
          </View>
        </View>
      ) : null}

      <Animated.View layout={LinearTransition.duration(200)} style={s.card}>
        {q && q.nights > 0 ? (
          <>
            {!custom ? (
              <>
                {q.lines.map((l, i) => (
                  <KV
                    key={l.label}
                    label={i === 0 ? `${l.label} from your rates` : l.label}
                    value={money(l.amount, cur)}
                  />
                ))}
                <KV label="Total" value={money(q.total, cur)} total />
              </>
            ) : (
              <Field
                label="Agreed total"
                value={customTotal}
                onChangeText={(t) => setCustomTotal(t.replace(/[^\d.]/g, ""))}
                keyboardType="decimal-pad"
                prefix={<Text style={s.prefix}>{symbol}</Text>}
                placeholder={(q.total / 100).toFixed(0)}
              />
            )}
            <View style={s.toggleRow}>
              <Text style={[ui.body, { flex: 1 }]}>Set the price myself</Text>
              <BrandSwitch
                label="Set the price myself"
                value={custom}
                onChange={(v) => {
                  setCustom(v);
                  if (v && !customTotal) setCustomTotal(String(Math.round(q.total / 100)));
                }}
              />
            </View>
          </>
        ) : (
          <Text style={ui.muted}>Pick dates to see the price from your rates.</Text>
        )}
        {!editing ? (
          <>
            <View style={s.toggleRow}>
              <View style={{ flex: 1 }}>
                <Text style={ui.body}>Deposit received</Text>
                {depositOn && Number(deposit) > 0 ? (
                  <Text style={ui.faint}>
                    {money(Math.round(Number(deposit) * 100), cur)} by{" "}
                    {METHOD_LABEL[method].toLowerCase()}
                  </Text>
                ) : null}
              </View>
              <BrandSwitch
                label="Deposit received"
                value={depositOn}
                onChange={(v) => {
                  setDepositOn(v);
                  if (v && !deposit)
                    setDeposit(String(Math.round(depositFor(property, total) / 100)));
                }}
              />
            </View>
            {depositOn ? (
              <Animated.View entering={FadeInDown.duration(200)} style={{ gap: 10, paddingTop: 4 }}>
                <Field
                  value={deposit}
                  onChangeText={(t) => setDeposit(t.replace(/[^\d.]/g, ""))}
                  keyboardType="decimal-pad"
                  prefix={<Text style={s.prefix}>{symbol}</Text>}
                />
                <View style={s.pills}>
                  {(["bank", "cash", "card"] as const).map((m) => (
                    <Pill
                      key={m}
                      label={METHOD_LABEL[m]}
                      selected={method === m}
                      onPress={() => setMethod(m)}
                    />
                  ))}
                </View>
              </Animated.View>
            ) : null}
          </>
        ) : null}
      </Animated.View>

      <Field
        label="Note for your team (optional)"
        value={note}
        onChangeText={setNote}
        placeholder="Leave pool towels out"
        multiline
      />
    </Page>
  );
}

function DateField({
  label,
  value,
  active,
  onPress,
}: {
  label: string;
  value: string | null;
  active: boolean;
  onPress: () => void;
}) {
  return (
    <Pressable
      onPress={() => {
        haptics.select();
        onPress();
      }}
      style={{ flex: 1, gap: 6 }}
    >
      <Text style={s.label}>{label}</Text>
      <View style={[s.dateBox, active && { borderColor: colors.ember, borderWidth: 1.5 }]}>
        <Text style={[ui.body, !value && { color: colors.ash }]}>
          {value ? formatShort(value) : "Select"}
        </Text>
      </View>
    </Pressable>
  );
}

const s = StyleSheet.create({
  pills: { flexDirection: "row", flexWrap: "wrap", gap: 8 },
  two: { flexDirection: "row", gap: 8 },
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 4,
  },
  label: { fontFamily: font.regular, fontSize: 13, color: colors.steel },
  dateBox: {
    height: 52,
    borderRadius: radius.input,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    justifyContent: "center",
    paddingHorizontal: 16,
  },
  paste: { fontFamily: font.medium, fontSize: 14, color: colors.steel },
  prefix: { fontFamily: font.medium, fontSize: 16, color: colors.steel },
  toggleRow: {
    flexDirection: "row",
    alignItems: "center",
    gap: 12,
    paddingTop: 12,
    marginTop: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
  },
});
