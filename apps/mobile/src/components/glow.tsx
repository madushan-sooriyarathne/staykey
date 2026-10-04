import { colors } from "@staykey/tokens";
import { useEffect, useId } from "react";
import type { StyleProp, ViewStyle } from "react-native";
import Animated, {
  Easing,
  useAnimatedStyle,
  useReducedMotion,
  useSharedValue,
  withRepeat,
  withTiming,
} from "react-native-reanimated";
import Svg, { Circle, Defs, RadialGradient, Stop } from "react-native-svg";

/**
 * The brand's one decorative accent: a soft Magenta Spark to Ember glow, used on hero cards.
 * It breathes slowly unless the system asks for reduced motion.
 */
export function Glow({
  size = 260,
  style,
  intensity = 1,
}: {
  size?: number;
  style?: StyleProp<ViewStyle>;
  intensity?: number;
}) {
  const id = useId().replace(/:/g, "");
  const reduceMotion = useReducedMotion();
  const breathe = useSharedValue(1);

  useEffect(() => {
    if (reduceMotion) return;
    breathe.value = withRepeat(
      withTiming(1.08, { duration: 3200, easing: Easing.inOut(Easing.sin) }),
      -1,
      true,
    );
  }, [breathe, reduceMotion]);

  const animated = useAnimatedStyle(() => ({ transform: [{ scale: breathe.value }] }));

  return (
    <Animated.View
      pointerEvents="none"
      style={[{ position: "absolute", width: size, height: size }, style, animated]}
    >
      <Svg width={size} height={size}>
        <Defs>
          <RadialGradient id={id} cx="50%" cy="50%" r="50%">
            <Stop offset="0" stopColor={colors.magenta} stopOpacity={0.55 * intensity} />
            <Stop offset="0.42" stopColor={colors.ember} stopOpacity={0.32 * intensity} />
            <Stop offset="1" stopColor={colors.ember} stopOpacity={0} />
          </RadialGradient>
        </Defs>
        <Circle cx={size / 2} cy={size / 2} r={size / 2} fill={`url(#${id})`} />
      </Svg>
    </Animated.View>
  );
}
