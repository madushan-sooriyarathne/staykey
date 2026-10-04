import { colors } from "@staykey/tokens";
import { useId } from "react";
import { StyleSheet } from "react-native";
import Svg, { Defs, Line, Pattern, Rect } from "react-native-svg";

/** Diagonal hatching for blocked nights, matching the calendar legend. */
export function Stripes({ color = colors.mist }: { color?: string }) {
  const id = `stripes${useId().replace(/:/g, "")}`;
  return (
    <Svg style={StyleSheet.absoluteFill} pointerEvents="none">
      <Defs>
        <Pattern
          id={id}
          width={8}
          height={8}
          patternUnits="userSpaceOnUse"
          patternTransform="rotate(45)"
        >
          <Line x1={0} y1={0} x2={0} y2={8} stroke={color} strokeWidth={3} />
        </Pattern>
      </Defs>
      <Rect width="100%" height="100%" fill={`url(#${id})`} />
    </Svg>
  );
}
