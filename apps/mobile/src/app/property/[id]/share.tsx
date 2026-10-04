import { colors, radius } from "@staykey/tokens";
import * as Clipboard from "expo-clipboard";
import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Platform, Share, StyleSheet, Text, View } from "react-native";
import QRCode from "react-native-qrcode-svg";
import Animated, { FadeIn } from "react-native-reanimated";
import { Button } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, Hint, Page, SectionHeader, Segmented, Two, ui } from "@/components/kit";
import { useProperty } from "@/data/hooks";
import { email, openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

/** Link, QR code and WhatsApp share, plus widget and iFrame code for the owner's website. */
export default function ShareAndEmbed() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const p = useProperty(id);
  const [mode, setMode] = useState<"widget" | "iframe">("widget");
  const [copied, setCopied] = useState<"link" | "code" | null>(null);
  if (!p) return null;
  const display = p.bookingPageUrl.replace(/^https?:\/\//, "");
  const code =
    mode === "widget"
      ? `<script src="https://cdn.staykey.direct/widget.js" async></script>\n<div data-staykey="${p.slug}"></div>`
      : `<iframe src="${p.bookingPageUrl}/embed" style="width:100%;border:0;min-height:640px" loading="lazy"></iframe>`;

  async function copy(what: "link" | "code", text: string) {
    await Clipboard.setStringAsync(text);
    haptics.select();
    setCopied(what);
    setTimeout(() => setCopied(null), 1600);
  }

  return (
    <Page title="Share and embed">
      <Card>
        <View style={s.qrRow}>
          <View style={s.qr}>
            <QRCode
              value={p.bookingPageUrl}
              size={96}
              color={colors.obsidian}
              backgroundColor={colors.snow}
            />
          </View>
          <View style={{ flex: 1, gap: 4 }}>
            <Text style={ui.strong}>{display}</Text>
            <Text style={ui.faint}>
              Print the QR code for the reception desk or your room cards.
            </Text>
          </View>
        </View>
        <View style={{ paddingTop: 12 }}>
          <Two>
            <Button
              compact
              title="WhatsApp"
              icon={I.chat}
              onPress={() =>
                openWhatsApp(undefined, `Book ${p.name} directly with us: ${p.bookingPageUrl}`)
              }
            />
            <Button
              compact
              variant="ghost"
              title={copied === "link" ? "Copied" : "Copy link"}
              icon={copied === "link" ? I.check : I.copy}
              onPress={() => copy("link", p.bookingPageUrl)}
            />
          </Two>
        </View>
        <View style={{ paddingTop: 8 }}>
          <Button
            compact
            variant="ghost"
            title="More ways to share"
            icon={I.share}
            onPress={() =>
              Share.share({ message: `Book ${p.name} directly with us: ${p.bookingPageUrl}` })
            }
          />
        </View>
      </Card>

      <SectionHeader title="Add to your website" />
      <Segmented
        options={[
          { id: "widget", label: "Booking widget" },
          { id: "iframe", label: "Full booking page" },
        ]}
        value={mode}
        onChange={setMode}
      />
      <Animated.View key={mode} entering={FadeIn.duration(220)} style={s.code}>
        <Text selectable style={s.codeText}>
          {code}
        </Text>
      </Animated.View>
      <Hint>
        {mode === "widget"
          ? "The widget shows your dates and prices inside your site and opens checkout in a window."
          : "The iFrame puts the whole booking page inside one page of your site."}
      </Hint>
      <Two>
        <Button
          compact
          variant="ghost"
          title={copied === "code" ? "Copied" : "Copy code"}
          icon={copied === "code" ? I.check : I.code}
          onPress={() => copy("code", code)}
        />
        <Button
          compact
          variant="ghost"
          title="Email developer"
          icon={I.mail}
          onPress={() =>
            email(
              "",
              `Booking widget for ${p.name}`,
              `Hi,\n\nCould you add our StayKey booking ${mode === "widget" ? "widget" : "page"} to the website? Paste this where it should appear:\n\n${code}\n\nThanks!`,
            )
          }
        />
      </Two>
    </Page>
  );
}

const s = StyleSheet.create({
  qrRow: { flexDirection: "row", alignItems: "center", gap: 14 },
  qr: {
    padding: 8,
    borderRadius: 16,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
  },
  code: { backgroundColor: colors.graphite, borderRadius: radius.card - 8, padding: 16 },
  codeText: {
    fontFamily: Platform.select({
      ios: "Menlo",
      android: "monospace",
      default: "ui-monospace, Menlo, monospace",
    }),
    fontSize: 12.5,
    lineHeight: 19,
    color: colors.mist,
  },
});
