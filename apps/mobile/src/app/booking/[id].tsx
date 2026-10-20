import { colors, radius } from "@staykey/tokens";
import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeIn, FadeInDown, LinearTransition } from "react-native-reanimated";
import { useEraseGuest } from "@/api/bookings";
import { messageFor } from "@/api/errors";
import { Tag } from "@/components/brand";
import { Button, InfoNote } from "@/components/controls";
import { I } from "@/components/icons";
import {
  Appear,
  Card,
  EmptyState,
  IconButton,
  KV,
  Menu,
  money,
  Page,
  PaymentTag,
  StatusTag,
  Two,
  ui,
} from "@/components/kit";
import { font } from "@/components/ui";
import { formatShort, nightsBetween, today } from "@/data/dates";
import { useBooking, useCan, useProperty } from "@/data/hooks";
import { clock, METHOD_LABEL, plural } from "@/data/labels";
import { balanceOf, isOTA, paidOf, SOURCE_LABEL } from "@/data/pricing";
import type { Booking, PropertyConfig } from "@/data/types";
import { type Next, useMoveStay } from "@/features/bookings/move";
import { guestLabel, placeLabel } from "@/features/bookings/rows";
import { call, email, openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

const ease = Easing.out(Easing.cubic);
/** The name the server gives a guest whose details were removed. */
const GUEST_REMOVED = "Guest removed";

export default function BookingDetail() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const booking = useBooking(id);
  const property = useProperty(booking?.propertyId);
  const can = useCan();
  const { move: moveStay, error } = useMoveStay();
  const [menu, setMenu] = useState(false);
  const [erasing, setErasing] = useState(false);
  const [eraseError, setEraseError] = useState<string | null>(null);
  const erase = useEraseGuest();

  if (!booking || !property) {
    return (
      <Page title="Booking">
        <EmptyState icon={I.calendar} title="Booking not found" body="It may have been removed." />
      </Page>
    );
  }

  const owner = can("prices");
  const b = booking;
  const ota = isOTA(b);
  const nights = nightsBetween(b.checkIn, b.checkOut);
  const guests = [
    plural(b.adults, "adult"),
    b.children ? plural(b.children, "child", "children") : "",
  ]
    .filter(Boolean)
    .join(", ");
  const day = today();
  const due = balanceOf(b);
  const extras = b.extras
    .map((x) => property.extras.find((e) => e.id === x)?.name)
    .filter(Boolean) as string[];
  const active = !["cancelled", "declined", "checked_out"].includes(b.status);

  function move(to: Next) {
    haptics.success();
    moveStay(b, to);
  }

  const push = (
    pathname: "/booking/payment" | "/booking/cancel" | "/booking/contact" | "/booking/new",
  ) => router.push({ pathname, params: { id: b.id } });

  const footer =
    ota || !active ? null : owner ? (
      <OwnerActions booking={b} due={due} onMove={move} onPush={push} />
    ) : (
      <StayActions booking={b} onMove={move} />
    );

  return (
    <Page
      title={b.ref}
      right={
        owner && !ota ? (
          <IconButton
            testID="booking-more"
            icon={I.more}
            label="More actions"
            onPress={() => setMenu(true)}
          />
        ) : undefined
      }
      footer={footer}
    >
      <Menu
        visible={menu}
        onClose={() => setMenu(false)}
        items={[
          { label: "Contact guest", icon: I.chat, onPress: () => push("/booking/contact") },
          ...(active
            ? [{ label: "Modify booking", icon: I.edit, onPress: () => push("/booking/new") }]
            : []),
          ...(active
            ? [{ label: "Record payment", icon: I.cash, onPress: () => push("/booking/payment") }]
            : []),
          ...(can("billing") && b.guest.name !== GUEST_REMOVED
            ? [
                {
                  label: "Remove guest's details",
                  icon: I.person,
                  destructive: true,
                  onPress: () => setErasing(true),
                  testID: "menu-erase-guest",
                },
              ]
            : []),
          ...(active
            ? [
                {
                  label: "Cancel booking",
                  icon: I.close,
                  destructive: true,
                  onPress: () => push("/booking/cancel"),
                  testID: "menu-cancel",
                },
              ]
            : []),
        ]}
      />

      {error || eraseError ? (
        <InfoNote icon={I.warning} testID="booking-error">
          {error ?? eraseError}
        </InfoNote>
      ) : null}

      {erasing ? (
        <Card title={`Remove ${b.guest.name}'s details?`}>
          <Text style={ui.muted}>
            Their name, phone, email and notes are removed from every stay they made with you, and
            Activity stops naming them. The stays and payments stay for your records. This can't be
            undone.
          </Text>
          <View style={{ paddingTop: 12 }}>
            <Two>
              <Button compact variant="ghost" title="Keep" onPress={() => setErasing(false)} />
              <Button
                compact
                title="Remove details"
                loading={erase.isPending}
                testID="erase-guest-confirm"
                onPress={() => {
                  setEraseError(null);
                  erase.mutate(b.id, {
                    onSuccess: () => {
                      haptics.success();
                      setErasing(false);
                    },
                    onError: (e) => {
                      haptics.error();
                      setEraseError(messageFor(e));
                    },
                  });
                }}
              />
            </Two>
          </View>
        </Card>
      ) : null}

      <Appear index={0} style={{ gap: 6 }}>
        <Animated.View key={b.status} entering={FadeIn.duration(220)} style={s.tags}>
          <StatusTag booking={b} />
          {owner && !ota && active && b.status !== "requested" ? (
            due === 0 ? (
              <Tag tone="spark" label="Paid in full" />
            ) : (
              <PaymentTag booking={b} currency={property.currency} />
            )
          ) : null}
          {ota ? <Tag tone="soft" label={`${SOURCE_LABEL[b.source]} via iCal`} /> : null}
        </Animated.View>
        <Text style={s.name}>{guestLabel(b)}</Text>
        <Text style={ui.muted}>
          {placeLabel(property, b, true)}, {plural(nights, "night")}
          {ota ? "" : `, ${guests}`}
        </Text>
      </Appear>

      {!ota ? (
        <Appear index={1} style={s.contact}>
          <View style={{ flex: 1 }}>
            <Button
              compact
              variant="ghost"
              title="WhatsApp"
              icon={I.chat}
              disabled={!b.guest.phone}
              onPress={() => openWhatsApp(b.guest.phone)}
            />
          </View>
          <View style={{ flex: 1 }}>
            <Button
              compact
              variant="ghost"
              title="Call"
              icon={I.phone}
              disabled={!b.guest.phone}
              onPress={() => b.guest.phone && call(b.guest.phone)}
            />
          </View>
          {owner ? (
            <View style={{ flex: 1 }}>
              <Button
                compact
                variant="ghost"
                title="Email"
                icon={I.mail}
                disabled={!b.guest.email}
                onPress={() =>
                  b.guest.email && email(b.guest.email, `Your stay at ${property.name}`)
                }
              />
            </View>
          ) : null}
        </Appear>
      ) : null}

      {b.status === "requested" ? (
        <Appear index={1}>
          <InfoNote icon={I.clock}>
            {b.guest.name.split(" ")[0]} asked to book. Approve to send the payment link, or decline
            to open the dates.
            {b.requestExpiresAt
              ? ` Declines on its own in ${plural(Math.max(1, Math.round((new Date(b.requestExpiresAt).getTime() - Date.now()) / 3_600_000)), "hour")}.`
              : ""}
          </InfoNote>
        </Appear>
      ) : null}

      <Appear index={2}>
        <Card>
          <View style={s.split}>
            <View style={{ flex: 1, gap: 2 }}>
              <Text style={ui.faint}>Check-in</Text>
              <Text style={s.date}>{formatShort(b.checkIn)}</Text>
              <Text style={ui.faint}>From {clock(property.checkIn)}</Text>
            </View>
            <View style={s.vr} />
            <View style={{ flex: 1, gap: 2 }}>
              <Text style={ui.faint}>Check-out</Text>
              <Text style={s.date}>{formatShort(b.checkOut)}</Text>
              <Text style={ui.faint}>By {clock(property.checkOut)}</Text>
            </View>
          </View>
          {b.status === "checked_in" ? (
            <Animated.View entering={FadeInDown.duration(220)} style={s.inHouse}>
              <Tag tone="spark" label="In house" pulse />
              <Text style={ui.faint}>
                {b.checkOut === day
                  ? "Leaves today"
                  : `Leaves in ${plural(nightsBetween(day, b.checkOut), "day")}`}
              </Text>
            </Animated.View>
          ) : null}
        </Card>
      </Appear>

      {extras.length || b.guestNote || b.ownerNote ? (
        <Appear index={3}>
          <Card title="For the stay">
            {extras.length ? <KV label="Extras" value={extras.join(", ")} /> : null}
            {b.guestNote ? <Note label="Guest note" text={b.guestNote} /> : null}
            {b.ownerNote ? <Note label="Owner note" text={b.ownerNote} /> : null}
          </Card>
        </Appear>
      ) : null}

      {ota ? (
        <Appear index={3}>
          <InfoNote icon={I.sync}>
            Imported from {SOURCE_LABEL[b.source]} by calendar sync. Guest details and payment stay
            in {SOURCE_LABEL[b.source]}, and these dates are closed on your booking page.
          </InfoNote>
        </Appear>
      ) : owner ? (
        <Appear index={4}>
          <PaymentCard booking={b} property={property} />
        </Appear>
      ) : (
        <Appear index={4}>
          <View style={s.lock}>
            <Text style={[ui.faint, { textAlign: "center" }]}>
              Prices and payments are visible to the owner and managers only.
            </Text>
          </View>
        </Appear>
      )}

      {b.cancel ? (
        <Appear index={5}>
          <Card title="Cancelled">
            <KV label="Reason" value={b.cancel.reason} />
            {owner ? <KV label="Refund" value={money(b.cancel.refund, property.currency)} /> : null}
          </Card>
        </Appear>
      ) : null}

      {owner && !ota ? (
        <Text style={[ui.faint, { textAlign: "center" }]}>
          Booked{" "}
          {new Date(b.createdAt).toLocaleDateString("en-GB", { day: "numeric", month: "short" })}{" "}
          through{" "}
          {["page", "widget", "phone", "walkin", "other"].includes(b.source)
            ? SOURCE_LABEL[b.source].toLowerCase()
            : SOURCE_LABEL[b.source]}
        </Text>
      ) : null}
    </Page>
  );
}

function Note({ label, text }: { label: string; text: string }) {
  return (
    <View style={s.note}>
      <Text style={s.noteLabel}>{label}</Text>
      <Text style={ui.body}>{text}</Text>
    </View>
  );
}

const PAYS = { bank: "By bank transfer", property: "At the property", card: "By card" } as const;

/** A payment's line: how and when, and for card refunds whether Payments.lk has sent them. */
function paymentLabel(p: Booking["payments"][number]): string {
  const how = p.method === "refund" && p.via === "card" ? "Card refund" : METHOD_LABEL[p.method];
  const when = new Date(p.at).toLocaleDateString("en-GB", { day: "numeric", month: "short" });
  const state =
    p.status === "pending"
      ? ", on its way"
      : p.status === "failed"
        ? ", failed; refund another way"
        : "";
  return `${how}, ${when}${state}`;
}

function PaymentCard({ booking: b, property }: { booking: Booking; property: PropertyConfig }) {
  const due = balanceOf(b);
  const paid = paidOf(b);
  const m = (n: number) => money(n, property.currency);
  return (
    <Animated.View layout={LinearTransition.duration(220)}>
      <Card
        title="Payment"
        meta={
          <Tag
            tone="ember"
            label={
              ["whatsapp", "phone", "walkin", "other"].includes(b.source)
                ? "Manual booking"
                : "Direct booking"
            }
          />
        }
      >
        {b.lines.map((l) => (
          <KV
            key={l.label}
            label={l.label}
            value={m(l.amount)}
            tone={l.kind === "discount" ? "spark" : undefined}
          />
        ))}
        <KV label="Total" value={m(b.total)} total />
        {b.paymentMethod ? <KV label="Pays" value={PAYS[b.paymentMethod]} muted /> : null}
        {b.payments.map((p) => (
          <KV
            key={p.id}
            muted
            label={paymentLabel(p)}
            value={`${p.method === "refund" ? "-" : ""}${m(p.amount)}${p.charged && property.currency !== "LKR" ? ` (${money(p.charged, "LKR")})` : ""}`}
            tone={p.status === "failed" ? "ember" : undefined}
          />
        ))}
        {b.status !== "cancelled" && b.status !== "declined" ? (
          due > 0 ? (
            <KV label={paid > 0 ? "Balance due" : "Due"} value={m(due)} tone="ember" />
          ) : (
            <KV label="Balance" value="Paid" tone="spark" />
          )
        ) : null}
        {b.slip?.status === "pending" ? (
          <View style={{ paddingTop: 8 }}>
            <Button
              compact
              variant="ghost"
              title={`Check bank slip for ${m(b.slip.amount)}`}
              icon={I.doc}
              onPress={() => router.push({ pathname: "/booking/payment", params: { id: b.id } })}
            />
          </View>
        ) : null}
      </Card>
    </Animated.View>
  );
}

function OwnerActions({
  booking: b,
  due,
  onMove,
  onPush,
}: {
  booking: Booking;
  due: number;
  onMove: (to: Next) => void;
  onPush: (p: "/booking/payment" | "/booking/cancel" | "/booking/contact" | "/booking/new") => void;
}) {
  const day = today();
  if (b.status === "requested") {
    return (
      <>
        <Flex>
          <Button
            variant="ghost"
            title="Decline"
            onPress={() => onMove("declined")}
            testID="detail-decline"
          />
        </Flex>
        <Flex>
          <Button
            title="Approve"
            onPress={() => onMove("awaiting_payment")}
            testID="detail-approve"
          />
        </Flex>
      </>
    );
  }
  const primary =
    b.status === "checked_in" ? (
      due > 0 ? (
        <Button
          title="Record payment"
          onPress={() => onPush("/booking/payment")}
          testID="detail-pay"
        />
      ) : (
        <Button title="Check out" onPress={() => onMove("checked_out")} testID="detail-checkout" />
      )
    ) : b.status === "confirmed" && b.checkIn <= day ? (
      <Button title="Check in" onPress={() => onMove("checked_in")} testID="detail-checkin" />
    ) : due > 0 ? (
      <Button
        title="Record payment"
        onPress={() => onPush("/booking/payment")}
        testID="detail-pay"
      />
    ) : (
      <Button title="Message guest" icon={I.chat} onPress={() => onPush("/booking/contact")} />
    );
  return (
    <>
      <Flex>
        <Button
          variant="ghost"
          title="Modify"
          onPress={() => onPush("/booking/new")}
          testID="detail-modify"
        />
      </Flex>
      <Flex>{primary}</Flex>
    </>
  );
}

function StayActions({ booking: b, onMove }: { booking: Booking; onMove: (to: Next) => void }) {
  const day = today();
  if (b.status === "checked_in")
    return (
      <Flex>
        <Button title="Check out" onPress={() => onMove("checked_out")} />
      </Flex>
    );
  if (b.status === "confirmed" && b.checkIn <= day)
    return (
      <Flex>
        <Button title="Check in" onPress={() => onMove("checked_in")} />
      </Flex>
    );
  return null;
}

function Flex({ children }: { children: React.ReactNode }) {
  return (
    <Animated.View entering={FadeIn.duration(200).easing(ease)} style={{ flex: 1 }}>
      {children}
    </Animated.View>
  );
}

const s = StyleSheet.create({
  tags: { flexDirection: "row", flexWrap: "wrap", gap: 6 },
  name: {
    fontFamily: font.semibold,
    fontSize: 28,
    lineHeight: 34,
    color: colors.obsidian,
    marginTop: 4,
  },
  contact: { flexDirection: "row", gap: 8 },
  split: { flexDirection: "row", gap: 16 },
  vr: { width: 1, backgroundColor: colors.cloud },
  date: { fontFamily: font.semibold, fontSize: 18, color: colors.obsidian },
  inHouse: {
    flexDirection: "row",
    alignItems: "center",
    gap: 8,
    borderTopWidth: 1,
    borderTopColor: colors.cloud,
    marginTop: 12,
    paddingTop: 12,
  },
  note: { backgroundColor: colors.paper, borderRadius: 16, padding: 12, gap: 2, marginTop: 6 },
  noteLabel: { fontFamily: font.medium, fontSize: 12, color: colors.fog },
  lock: {
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    borderStyle: "dashed",
    padding: 16,
  },
});
