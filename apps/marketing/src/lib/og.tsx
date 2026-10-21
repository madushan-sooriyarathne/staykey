import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { ImageResponse } from "next/og";

export const ogSize = { width: 1200, height: 630 };

// Colours from @staykey/tokens; the image renderer can't read CSS variables.
const OBSIDIAN = "#09090b";
const STEEL = "#52525b";
const PAPER = "#f4f4f5";
const CLOUD = "#ececee";
const EMBER = "#ff5a00";
const EMBER_TINT = "#fff0e6";
const EMBER_INK = "#b83f00";

/** A shared-link image: the wordmark, a headline, a line under it and a chip. */
export async function ogImage({
  headline,
  line,
  chip,
}: {
  headline: string;
  line: string;
  chip: string;
}) {
  const [regular, semibold] = await Promise.all([
    readFile(join(process.cwd(), "assets/fonts/dm-sans-latin-400-normal.woff")),
    readFile(join(process.cwd(), "assets/fonts/dm-sans-latin-600-normal.woff")),
  ]);
  return new ImageResponse(
    <div
      style={{
        width: "100%",
        height: "100%",
        display: "flex",
        flexDirection: "column",
        justifyContent: "space-between",
        padding: 72,
        background: PAPER,
        fontFamily: "DM Sans",
      }}
    >
      <div style={{ display: "flex", alignItems: "center", gap: 16 }}>
        <div
          style={{
            width: 56,
            height: 56,
            borderRadius: 14,
            background: OBSIDIAN,
            color: "white",
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            fontSize: 36,
            fontWeight: 600,
          }}
        >
          S
        </div>
        <span style={{ fontSize: 34, fontWeight: 600, color: OBSIDIAN }}>StayKey</span>
      </div>
      <div style={{ display: "flex", flexDirection: "column", gap: 24 }}>
        <span
          style={{
            fontSize: 76,
            fontWeight: 600,
            color: OBSIDIAN,
            lineHeight: 1.08,
            maxWidth: 980,
          }}
        >
          {headline}
        </span>
        <span style={{ fontSize: 32, color: STEEL, maxWidth: 900 }}>{line}</span>
      </div>
      <div style={{ display: "flex", alignItems: "center", justifyContent: "space-between" }}>
        <span
          style={{
            display: "flex",
            alignItems: "center",
            gap: 12,
            fontSize: 26,
            color: EMBER_INK,
            background: EMBER_TINT,
            borderRadius: 12,
            padding: "10px 18px",
          }}
        >
          <span style={{ width: 12, height: 12, borderRadius: 6, background: EMBER }} />
          {chip}
        </span>
        <span style={{ fontSize: 26, color: STEEL, borderBottom: `2px solid ${CLOUD}` }}>
          staykey.direct
        </span>
      </div>
    </div>,
    {
      ...ogSize,
      fonts: [
        { name: "DM Sans", data: regular, weight: 400, style: "normal" },
        { name: "DM Sans", data: semibold, weight: 600, style: "normal" },
      ],
    },
  );
}
