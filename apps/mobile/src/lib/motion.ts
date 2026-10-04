import {
  Easing,
  type EntryExitAnimationFunction,
  FadeIn,
  FadeInLeft,
  FadeInRight,
  FadeOut,
  type SharedValue,
  withSpring,
  withTiming,
} from "react-native-reanimated";

/**
 * Motion tokens. Springs are tuned to settle quickly without visible overshoot, which keeps
 * movement calm next to the brand's flat, hairline look.
 */
export const spring = {
  /** Step and page movement. */
  page: { damping: 26, stiffness: 240, mass: 0.9 },
  /** Small controls: buttons, cards, pills. */
  control: { damping: 18, stiffness: 320, mass: 0.6 },
  /** Progress bars and sliders. */
  bar: { damping: 22, stiffness: 160, mass: 1 },
} as const;

export const duration = {
  fadeIn: 240,
  fadeOut: 150,
  exit: 200,
} as const;

const TRAVEL = 44;

/**
 * Entering animation for an onboarding step. `direction` is read when the animation starts,
 * so forward steps slide in from the right and backward steps from the left.
 */
export function stepEntering(
  direction: SharedValue<number>,
  reduceMotion: boolean,
): EntryExitAnimationFunction {
  return () => {
    "worklet";
    const offset = reduceMotion ? 0 : TRAVEL * direction.value;
    return {
      initialValues: { opacity: 0, transform: [{ translateX: offset }] },
      animations: {
        opacity: withTiming(1, { duration: duration.fadeIn, easing: Easing.out(Easing.quad) }),
        transform: [{ translateX: withSpring(0, spring.page) }],
      },
    };
  };
}

/** Exit counterpart of stepEntering: the old step drifts the opposite way and fades. */
export function stepExiting(
  direction: SharedValue<number>,
  reduceMotion: boolean,
): EntryExitAnimationFunction {
  return () => {
    "worklet";
    const offset = reduceMotion ? 0 : -0.6 * TRAVEL * direction.value;
    return {
      initialValues: { opacity: 1, transform: [{ translateX: 0 }] },
      animations: {
        opacity: withTiming(0, { duration: duration.fadeOut, easing: Easing.in(Easing.quad) }),
        transform: [
          {
            translateX: withTiming(offset, {
              duration: duration.exit,
              easing: Easing.in(Easing.quad),
            }),
          },
        ],
      },
    };
  };
}

/**
 * Web can't run custom layout animation worklets, so the web build uses the closest predefined
 * pair: the new step slides in from the direction of travel and the old one fades out.
 */
export function webStepEntering(direction: 1 | -1, reduceMotion: boolean) {
  if (reduceMotion) return FadeIn.duration(duration.fadeIn);
  return (direction > 0 ? FadeInRight : FadeInLeft).duration(280).easing(Easing.out(Easing.cubic));
}

export const webStepExiting = FadeOut.duration(duration.fadeOut);
