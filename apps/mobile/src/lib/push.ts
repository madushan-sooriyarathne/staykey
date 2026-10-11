import Constants from "expo-constants";
import * as Notifications from "expo-notifications";
import { router } from "expo-router";
import { useEffect } from "react";
import { Platform } from "react-native";
import { api } from "@/api/client";
import { unwrap } from "@/api/errors";
import { useSession } from "@/lib/session";

// Alerts show while the app is open too.
Notifications.setNotificationHandler({
  handleNotification: async () => ({
    shouldShowBanner: true,
    shouldShowList: true,
    shouldPlaySound: true,
    shouldSetBadge: false,
  }),
});

let warned = false;

/**
 * Asks for permission and registers this device's Expo push token with the signed-in session,
 * so the team's alerts reach it. Skipped on web, and in builds without an EAS project id, which
 * Expo needs to issue a token.
 */
export async function registerForPush(): Promise<void> {
  if (Platform.OS === "web") return;
  const projectId = Constants.expoConfig?.extra?.eas?.projectId;
  if (typeof projectId !== "string" || projectId === "") {
    if (!warned)
      console.warn("Push alerts are off: run `bunx eas-cli init` to add an EAS project id.");
    warned = true;
    return;
  }
  if (Platform.OS === "android") {
    await Notifications.setNotificationChannelAsync("default", {
      name: "Alerts",
      importance: Notifications.AndroidImportance.HIGH,
    });
  }
  let { granted } = await Notifications.getPermissionsAsync();
  if (!granted) granted = (await Notifications.requestPermissionsAsync()).granted;
  if (!granted) return;
  try {
    const { data: token } = await Notifications.getExpoPushTokenAsync({ projectId });
    await unwrap(
      api.POST("/v1/push-tokens", {
        body: { token, platform: Platform.OS === "ios" ? "ios" : "android" },
      }),
    );
  } catch {
    // Offline or Expo unreachable: the next launch tries again.
  }
}

/** Where a tapped alert goes: the stay it is about, otherwise Activity. */
function open(response: Notifications.NotificationResponse) {
  const data = response.notification.request.content.data;
  const accountId = typeof data?.accountId === "string" ? data.accountId : undefined;
  const bookingId = typeof data?.bookingId === "string" ? data.bookingId : undefined;
  // An alert from another account opens Activity rather than a stay this account can't see.
  if (bookingId && (!accountId || accountId === useSession.getState().accountId)) {
    router.push({ pathname: "/booking/[id]", params: { id: bookingId } });
    return;
  }
  router.push("/activity");
}

/**
 * Registers for push once signed in and routes tapped alerts. Mount once, inside the signed-in
 * part of the app.
 */
export function usePushAlerts(enabled: boolean) {
  useEffect(() => {
    if (!enabled || Platform.OS === "web") return;
    registerForPush();
    const last = Notifications.getLastNotificationResponse();
    if (last) {
      open(last);
      Notifications.clearLastNotificationResponse();
    }
    const sub = Notifications.addNotificationResponseReceivedListener(open);
    return () => sub.remove();
  }, [enabled]);
}
