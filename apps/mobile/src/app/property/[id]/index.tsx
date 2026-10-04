import { colors, radius } from "@staykey/tokens";
import * as Clipboard from "expo-clipboard";
import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Image, Linking, Pressable, Share, StyleSheet, Text } from "react-native";
import Animated, { FadeIn } from "react-native-reanimated";
import { Tag } from "@/components/brand";
import { Button } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Card, IconButton, List, ListRow, money, Page, Two, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { useProperty } from "@/data/hooks";
import { CHANNEL_LABEL, plural } from "@/data/labels";
import { POLICY_TEXT } from "@/data/pricing";
import type { PropertyConfig } from "@/data/types";
import { haptics } from "@/lib/haptics";

type Href = Parameters<typeof router.push>[0];

/** Booking page link and share actions, with every setting below. Issues show on their row. */
export default function PropertyOverview() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const p = useProperty(id);
  const [copied, setCopied] = useState(false);
  if (!p) return null;

  const go = (screen: string) => () => router.push(`/property/${p.id}/${screen}` as Href);
  const display = p.bookingPageUrl.replace(/^https?:\/\//, "");
  const issues = p.ical.filter((f) => f.status === "error").length;
  const unsynced = p.ical.filter((f) => !f.url).length;
  const minRate = Math.min(...p.units.map((u) => u.rate));
  const activePromos = p.promos.filter(
    (x) => !x.to || x.to >= new Date().toISOString().slice(0, 10),
  ).length;

  return (
    <Page
      title={p.name}
      right={
        <IconButton
          icon={I.eye}
          label="Open booking page"
          onPress={() => Linking.openURL(p.bookingPageUrl)}
        />
      }
    >
      <Appear index={0}>
        <Pressable onPress={go("photos")} style={s.cover}>
          {p.photos[0] ? (
            <Image source={{ uri: p.photos[0].uri }} style={StyleSheet.absoluteFill} />
          ) : (
            <Text style={ui.faint}>Add a cover photo</Text>
          )}
        </Pressable>
      </Appear>

      <Appear index={1}>
        <Card title="Booking page" meta={<Tag tone="spark" label="Live" pulse />}>
          <Text style={[ui.body, { paddingBottom: 10 }]}>{display}</Text>
          <Two>
            <Button
              compact
              variant="ghost"
              title={copied ? "Copied" : "Copy link"}
              icon={copied ? I.check : I.copy}
              onPress={async () => {
                await Clipboard.setStringAsync(p.bookingPageUrl);
                haptics.select();
                setCopied(true);
                setTimeout(() => setCopied(false), 1600);
              }}
            />
            <Button
              compact
              variant="ghost"
              title="Share"
              icon={I.share}
              onPress={() =>
                Share.share({ message: `Book ${p.name} directly with us: ${p.bookingPageUrl}` })
              }
            />
          </Two>
        </Card>
      </Appear>

      <Appear index={2}>
        <List title="Property">
          <ListRow
            testID="prop-details"
            icon={I.house}
            title="Details"
            subtitle={p.location || "Add a location"}
            onPress={go("details")}
          />
          <ListRow
            testID="prop-photos"
            icon={I.photo}
            title="Photos"
            value={String(p.photos.length)}
            onPress={go("photos")}
          />
          <ListRow
            testID="prop-units"
            icon={I.bed}
            title="Units"
            value={String(p.units.length)}
            onPress={go("units")}
          />
        </List>
      </Appear>

      <Appear index={3}>
        <List title="Pricing">
          <ListRow
            testID="prop-rates"
            icon={I.tag}
            title="Rates and seasons"
            subtitle={`From ${money(minRate, p.currency)}${p.seasons.length ? `, ${plural(p.seasons.length, "season")}` : ""}`}
            onPress={go("rates")}
          />
          <ListRow
            testID="prop-rules"
            icon={I.rules}
            title="Stay rules"
            subtitle={`${plural(p.rules.minNights, "night")} minimum`}
            onPress={go("rules")}
          />
          <ListRow
            testID="prop-taxes"
            icon={I.percent}
            title="Taxes and charges"
            subtitle={
              p.charges
                .filter((c) => c.enabled)
                .map((c) => c.name)
                .join(", ") || "None added"
            }
            onPress={go("taxes")}
          />
          <ListRow
            testID="prop-extras"
            icon={I.gift}
            title="Extras"
            value={String(p.extras.filter((e) => e.enabled).length)}
            onPress={go("extras")}
          />
          <ListRow
            testID="prop-promos"
            icon={I.ticket}
            title="Promo codes"
            value={String(activePromos)}
            onPress={go("promos")}
          />
        </List>
      </Appear>

      <Appear index={4}>
        <List title="Guests and payments">
          <ListRow
            testID="prop-policies"
            icon={I.doc}
            title="Policies"
            subtitle={`${POLICY_TEXT[p.policy].label} cancellation, ${p.depositPercent}% deposit`}
            onPress={go("policies")}
          />
          <ListRow
            testID="prop-payments"
            icon={I.card}
            title="Payment methods"
            subtitle={paymentSummary(p)}
            onPress={go("payments")}
          />
          <ListRow
            testID="prop-booking"
            icon={I.settings}
            title="Booking settings"
            subtitle={p.booking.mode === "instant" ? "Instant book" : "Request to book"}
            onPress={go("booking")}
          />
        </List>
      </Appear>

      <Appear index={5}>
        <List title="Booking page">
          <ListRow
            testID="prop-branding"
            icon={I.brush}
            title="Booking page and branding"
            onPress={go("branding")}
          />
          <ListRow testID="prop-share" icon={I.qr} title="Share and embed" onPress={go("share")} />
          <ListRow
            testID="prop-ical"
            icon={I.sync}
            title="iCal sync"
            subtitle={
              p.ical.length
                ? p.ical.map((f) => CHANNEL_LABEL[f.channel]).join(", ")
                : "No calendars imported"
            }
            trailing={
              issues ? (
                <Animated.View entering={FadeIn}>
                  <Tag tone="ember" label={plural(issues, "issue")} />
                </Animated.View>
              ) : unsynced ? (
                <Tag tone="ember" label="Not connected" />
              ) : undefined
            }
            onPress={go("ical")}
          />
        </List>
      </Appear>
    </Page>
  );
}

function paymentSummary(p: PropertyConfig): string {
  const parts = [
    p.payments.bank.enabled && "Bank transfer",
    p.payments.atProperty && "pay at property",
    p.payments.cards === "on" && "cards",
  ].filter(Boolean);
  const text = parts.join(", ");
  return text ? text[0]?.toUpperCase() + text.slice(1) : "None turned on";
}

const s = StyleSheet.create({
  cover: {
    height: 180,
    borderRadius: radius.card,
    backgroundColor: colors.mist,
    overflow: "hidden",
    justifyContent: "flex-end",
    padding: 16,
  },
  label: { fontFamily: font.medium },
});
