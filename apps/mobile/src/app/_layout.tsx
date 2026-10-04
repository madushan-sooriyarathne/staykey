import {
  DMSans_400Regular,
  DMSans_500Medium,
  DMSans_600SemiBold,
  useFonts,
} from "@expo-google-fonts/dm-sans";
import { colors } from "@staykey/tokens";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { useEffect, useState } from "react";
import { useOnboarding } from "@/features/onboarding/store";
import { useSession } from "@/lib/session";

export { ErrorBoundary } from "expo-router";

SplashScreen.preventAutoHideAsync();

/** True once the onboarding draft has loaded from storage, so Welcome knows whether to resume. */
function useDraftHydrated() {
  const [hydrated, setHydrated] = useState(() => useOnboarding.persist.hasHydrated());
  useEffect(() => {
    if (hydrated) return;
    const unsub = useOnboarding.persist.onFinishHydration(() => setHydrated(true));
    if (useOnboarding.persist.hasHydrated()) setHydrated(true);
    return unsub;
  }, [hydrated]);
  return hydrated;
}

export default function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    DMSans_400Regular,
    DMSans_500Medium,
    DMSans_600SemiBold,
  });
  const sessionHydrated = useSession((s) => s.hydrated);
  const onboarded = useSession((s) => s.onboarded);
  const draftHydrated = useDraftHydrated();
  const ready = fontsLoaded && sessionHydrated && draftHydrated;

  useEffect(() => {
    if (fontError) throw fontError;
  }, [fontError]);

  useEffect(() => {
    if (ready) SplashScreen.hideAsync();
  }, [ready]);

  if (!ready) return null;

  return (
    <>
      <StatusBar style="dark" />
      <Stack
        screenOptions={{ contentStyle: { backgroundColor: colors.paper }, headerShown: false }}
      >
        {/* Signed out or still setting up. Finishing onboarding flips the guard and lands on Today. */}
        <Stack.Protected guard={!onboarded}>
          <Stack.Screen name="welcome" />
          <Stack.Screen name="onboarding" options={{ gestureEnabled: false }} />
          <Stack.Screen name="live" options={{ animation: "fade", gestureEnabled: false }} />
          <Stack.Screen name="join" />
        </Stack.Protected>

        <Stack.Protected guard={onboarded}>
          <Stack.Screen name="(tabs)" />
          <Stack.Screen
            name="property/new"
            options={{
              presentation: "modal",
              headerShown: true,
              title: "New property",
              headerShadowVisible: false,
            }}
          />
          <Stack.Screen name="paywall" options={{ presentation: "modal" }} />
        </Stack.Protected>
      </Stack>
    </>
  );
}
