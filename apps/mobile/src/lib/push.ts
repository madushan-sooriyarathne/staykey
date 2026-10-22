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
let failed = false;
let deferred = false;

/** "Not now" on the go-live screen: the app doesn't ask on its own again until it next starts. */
export function deferPushPrompt() {
  deferred = true;
}

/**
 * Asks for permission, unless the prompt was put off, and registers this device's Expo push
 * token with the signed-in session, so the team's alerts reach it. Resolves to whether alerts
 * are allowed, and never rejects. Skipped on web; builds without an EAS project id, which Expo
 * needs to issue a token, ask but don't register.
 */
export async function registerForPush(): Promise<boolean> {
  if (Platform.OS === "web") return false;
  let granted = false;
  try {
    // Android 13 shows the permission prompt only once a channel exists.
    if (Platform.OS === "android") {
      await Notifications.setNotificationChannelAsync("default", {
        name: "Alerts",
        importance: Notifications.AndroidImportance.HIGH,
      });
    }
    granted = (await Notifications.getPermissionsAsync()).granted;
    if (!granted && !deferred) granted = (await Notifications.requestPermissionsAsync()).granted;
    if (!granted) return false;
    const projectId = Constants.expoConfig?.extra?.eas?.projectId;
    if (typeof projectId !== "string" || projectId === "") {
      if (!warned)
        console.warn("Push alerts are off: run `bunx eas-cli init` to add an EAS project id.");
      warned = true;
      return true;
    }
    const { data: token } = await Notifications.getExpoPushTokenAsync({ projectId });
    await unwrap(
      api.POST("/v1/push-tokens", {
        body: { token, platform: Platform.OS === "ios" ? "ios" : "android" },
      }),
    );
  } catch (error) {
    // Offline, Expo unreachable or no notification support on this device: the next launch tries
    // again, so one warning is enough.
    if (!failed) console.warn("Couldn't register for push alerts", error);
    failed = true;
  }
  return granted;
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
    // Never rejects: failures are caught and warned about inside.
    void registerForPush();
    const last = Notifications.getLastNotificationResponse();
    if (last) {
      open(last);
      Notifications.clearLastNotificationResponse();
    }
    const sub = Notifications.addNotificationResponseReceivedListener(open);
    return () => sub.remove();
  }, [enabled]);
}
