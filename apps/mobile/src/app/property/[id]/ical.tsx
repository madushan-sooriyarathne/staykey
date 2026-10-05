import { colors, radius } from "@staykey/tokens";
import { useQueryClient } from "@tanstack/react-query";
import * as Clipboard from "expo-clipboard";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, { FadeIn, FadeInDown, LinearTransition } from "react-native-reanimated";
import { propertyKeys } from "@/api/properties";
import { IconBox, Tag } from "@/components/brand";
import { Button, Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Hint, Page, SectionHeader, Two, ui } from "@/components/kit";
import { font } from "@/components/ui";
import { formatClock } from "@/data/dates";
import { uid } from "@/data/defaults";
import { useProperty } from "@/data/hooks";
import { CHANNEL_LABEL, plural } from "@/data/labels";
import type { IcalFeed } from "@/data/types";
import { useLiveSave } from "@/features/property/settings";
import { haptics } from "@/lib/haptics";

/** Imported OTA calendars with status and errors, plus the export link for each OTA. */
export default function IcalSync() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const p = useProperty(id);
  const live = useLiveSave(id);
  const client = useQueryClient();
  const [syncing, setSyncing] = useState<string | null>(null);
  const [editing, setEditing] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [copied, setCopied] = useState(false);
  if (!p) return null;
  const exportUrl = `https://api.staykey.direct/ical/${p.slug}-${p.icalExportToken}.ics`;

  /** Saves a feed's link. The import worker reads it on its next run and reports back. */
  async function connect(feed: IcalFeed, url: string) {
    if (!p) return;
    setSyncing(feed.id);
    const saved = await live.apply({
      ical: p.ical.map((f) => (f.id === feed.id ? { ...f, url } : f)),
    });
    setSyncing(null);
    if (saved) {
      haptics.success();
      setEditing(null);
    }
  }

  /** Fetches the latest sync status from the server. */
  async function refresh(feed: IcalFeed) {
    setSyncing(feed.id);
    await client.invalidateQueries({ queryKey: propertyKeys.all });
    setSyncing(null);
  }

  return (
    <Page title="iCal sync">
      {live.error ? <InfoNote icon={I.warning}>{live.error}</InfoNote> : null}
      <SectionHeader title="Calendars you import" />
      {p.ical.map((f, i) => (
        <Appear key={f.id} index={i}>
          <Animated.View layout={LinearTransition.duration(220)} style={s.card}>
            <View style={s.row}>
              <IconBox
                name={I.calendar}
                tone={f.status === "error" ? "ember" : "neutral"}
                size={40}
              />
              <View style={{ flex: 1, gap: 2 }}>
                <Text style={ui.strong}>{CHANNEL_LABEL[f.channel]}</Text>
                <Text style={ui.faint}>
                  {f.status === "error"
                    ? `${f.error ?? "Sync failed"}, tried ${f.lastSync ? formatClock(f.lastSync) : "recently"}`
                    : f.status === "ok"
                      ? `Synced ${f.lastSync ? formatClock(f.lastSync) : "just now"}, ${plural(f.upcoming, "upcoming stay")}`
                      : f.url
                        ? "Link saved, waiting for the first sync"
                        : "Paste your calendar link to connect"}
                </Text>
              </View>
              {f.status === "error" ? (
                <Tag tone="ember" label="Issue" />
              ) : f.status === "ok" ? (
                <Tag tone="spark" label="Synced" />
              ) : null}
            </View>
            {(f.status === "pending" && !f.url) || editing === f.id ? (
              <Animated.View entering={FadeInDown.duration(200)} style={{ gap: 10 }}>
                <FeedLink feed={f} busy={syncing === f.id} onConnect={(url) => connect(f, url)} />
                <Hint>
                  In {CHANNEL_LABEL[f.channel]}, open your calendar settings and copy the export or
                  iCal link.
                </Hint>
              </Animated.View>
            ) : (
              <Two>
                <Button
                  compact
                  variant="ghost"
                  title="Check link"
                  onPress={() => setEditing(f.id)}
                />
                <Button
                  compact
                  title={f.status === "pending" ? "Check status" : "Sync now"}
                  loading={syncing === f.id}
                  onPress={() => refresh(f)}
                  testID={`sync-${f.channel}`}
                />
              </Two>
            )}
          </Animated.View>
        </Appear>
      ))}
      {adding ? (
        <AddFeed
          onCancel={() => setAdding(false)}
          onAdd={async (channel, url) => {
            const feed: IcalFeed = {
              id: uid("ical"),
              channel,
              url,
              status: "pending",
              upcoming: 0,
            };
            if (await live.apply({ ical: [...p.ical, feed] })) {
              haptics.success();
              setAdding(false);
            }
          }}
        />
      ) : (
        <Button
          compact
          variant="ghost"
          title="Add a calendar link"
          icon={I.plus}
          onPress={() => setAdding(true)}
        />
      )}

      <SectionHeader title="Your calendar for OTAs" />
      <View style={s.card}>
        <Text style={ui.muted}>
          Paste this link into Airbnb and Booking.com so they close the dates booked here.
        </Text>
        <View style={s.export}>
          <Text style={s.exportText} numberOfLines={1} selectable>
            {exportUrl.replace("https://", "")}
          </Text>
        </View>
        <Button
          compact
          variant="ghost"
          title={copied ? "Copied" : "Copy link"}
          icon={copied ? I.check : I.copy}
          onPress={async () => {
            await Clipboard.setStringAsync(exportUrl);
            haptics.select();
            setCopied(true);
            setTimeout(() => setCopied(false), 1600);
          }}
        />
      </View>
      <Hint>OTAs refresh imported calendars on their own schedule, often every few hours.</Hint>
    </Page>
  );
}

function FeedLink({
  feed,
  busy,
  onConnect,
}: {
  feed: IcalFeed;
  busy: boolean;
  onConnect: (url: string) => void;
}) {
  const [url, setUrl] = useState(feed.url);
  return (
    <View style={{ gap: 8 }}>
      <Field
        testID={`ical-url-${feed.channel}`}
        value={url}
        onChangeText={setUrl}
        autoCapitalize="none"
        autoCorrect={false}
        placeholder={`https://${feed.channel === "airbnb" ? "www.airbnb.com/calendar/ical/" : "admin.booking.com/"}...`}
      />
      <Button
        compact
        title="Connect"
        loading={busy}
        disabled={url.trim().length < 10}
        onPress={() => onConnect(url.trim())}
        testID={`ical-connect-${feed.channel}`}
      />
    </View>
  );
}

function AddFeed({
  onCancel,
  onAdd,
}: {
  onCancel: () => void;
  onAdd: (c: IcalFeed["channel"], url: string) => void;
}) {
  const [channel, setChannel] = useState<IcalFeed["channel"]>("airbnb");
  const [url, setUrl] = useState("");
  return (
    <Animated.View
      entering={FadeIn.duration(220)}
      style={[s.card, { borderColor: colors.obsidian, borderWidth: 1.5 }]}
    >
      <Text style={ui.strong}>Add a calendar</Text>
      <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
        {(Object.keys(CHANNEL_LABEL) as IcalFeed["channel"][]).map((c) => (
          <Pill
            key={c}
            label={CHANNEL_LABEL[c]}
            selected={channel === c}
            onPress={() => setChannel(c)}
          />
        ))}
      </View>
      <Field
        value={url}
        onChangeText={setUrl}
        autoCapitalize="none"
        autoCorrect={false}
        placeholder="Calendar link ending in .ics"
      />
      <Two>
        <Button compact variant="ghost" title="Cancel" onPress={onCancel} />
        <Button
          compact
          title="Add"
          disabled={url.trim().length < 10}
          onPress={() => onAdd(channel, url.trim())}
        />
      </Two>
    </Animated.View>
  );
}

const s = StyleSheet.create({
  card: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1,
    borderColor: colors.cloud,
    padding: 16,
    gap: 12,
  },
  row: { flexDirection: "row", alignItems: "center", gap: 12 },
  export: {
    backgroundColor: colors.paper,
    borderRadius: radius.input,
    paddingHorizontal: 14,
    paddingVertical: 14,
  },
  exportText: { fontFamily: font.medium, fontSize: 14, color: colors.graphite },
});
