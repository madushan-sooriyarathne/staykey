/**
 * StayKey brand tokens, shared by the mobile app (React Native) and the web apps.
 * Source of truth: the brand style reference (design.md). Web apps get the same
 * values as Tailwind theme variables from `@staykey/tokens/theme.css`.
 */

export const colors = {
  obsidian: "#09090b",
  graphite: "#18181b",
  slate: "#27272a",
  iron: "#3f3f46",
  steel: "#52525b",
  fog: "#71717a",
  ash: "#a1a1aa",
  mist: "#d4d4d8",
  cloud: "#ececee",
  paper: "#f4f4f5",
  subtle: "#fafafa",
  snow: "#ffffff",

  /** Direct bookings and anything that needs the owner's action. */
  ember: "#ff5a00",
  emberTint: "#fff0e6",
  emberInk: "#b83f00",

  /** Wins and live status. Use sparingly. */
  magenta: "#fe45e2",
  magentaTint: "#ffedfc",
  magentaInk: "#a3128c",
} as const;

export const semantic = {
  background: colors.paper,
  surface: colors.snow,
  surfaceSubtle: colors.subtle,
  surfaceDark: colors.graphite,
  border: colors.cloud,
  text: colors.obsidian,
  textBody: colors.graphite,
  textMuted: colors.steel,
  textFaint: colors.fog,
  placeholder: colors.ash,
  primary: colors.obsidian,
  onPrimary: colors.snow,
  direct: colors.ember,
  attention: colors.ember,
  success: colors.magenta,
} as const;

export const radius = {
  badge: 12,
  button: 14,
  input: 14,
  card: 28,
  cardLarge: 36,
  icon: 40,
  pill: 10000,
} as const;

export const spacing = {
  4: 4,
  8: 8,
  12: 12,
  16: 16,
  20: 20,
  24: 24,
  28: 28,
  32: 32,
  40: 40,
  48: 48,
  64: 64,
  80: 80,
} as const;

export const fontFamily = {
  /** Cosmica when licensed and installed, DM Sans otherwise. */
  sans: "Cosmica",
  fallback: "DM Sans",
} as const;

export const fontSize = {
  caption: 12,
  small: 13,
  body: 15,
  bodyLarge: 18,
  subheading: 20,
  screenTitle: 30,
  headingSmall: 32,
  heading: 40,
  headingLarge: 56,
  display: 64,
} as const;

export const fontWeight = {
  regular: "400",
  medium: "500",
  semibold: "600",
  bold: "700",
} as const;

export const tokens = {
  colors,
  semantic,
  radius,
  spacing,
  fontFamily,
  fontSize,
  fontWeight,
} as const;

export type ColorName = keyof typeof colors;
