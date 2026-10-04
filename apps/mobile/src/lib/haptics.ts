import * as Haptics from "expo-haptics";
import { Platform } from "react-native";

/**
 * StayKey's haptic vocabulary. Keep it small and subtle so feedback stays meaningful:
 *   step      light tap when moving forward a step
 *   back      selection tick when going back
 *   select    selection tick for choices, toggles and steppers
 *   success   publishing, verifying, confirming
 *   warning   something needs attention but nothing failed
 *   error     a rejected code or a failed save
 */
const enabled = Platform.OS === "ios" || Platform.OS === "android";

function run(fn: () => Promise<void>) {
  if (enabled) fn().catch(() => {});
}

export const haptics = {
  step: () => run(() => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Light)),
  back: () => run(() => Haptics.selectionAsync()),
  select: () => run(() => Haptics.selectionAsync()),
  press: () => run(() => Haptics.impactAsync(Haptics.ImpactFeedbackStyle.Soft)),
  success: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Success)),
  warning: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Warning)),
  error: () => run(() => Haptics.notificationAsync(Haptics.NotificationFeedbackType.Error)),
};
