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
import { useData } from "@/data/store";
import { useOnboarding } from "@/features/onboarding/store";
import { useSession } from "@/lib/session";

export { ErrorBoundary } from "expo-router";

SplashScreen.preventAutoHideAsync();

type Persisted = {
  persist: { hasHydrated: () => boolean; onFinishHydration: (fn: () => void) => () => void };
};

/**
 * True once a persisted store has loaded from storage. The root waits for the onboarding draft
 * (so Welcome knows whether to resume) and the owner's data (so screens open with real values).
 */
function useHydrated(store: Persisted) {
  const [hydrated, setHydrated] = useState(() => store.persist.hasHydrated());
  useEffect(() => {
    if (hydrated) return;
    const unsub = store.persist.onFinishHydration(() => setHydrated(true));
    if (store.persist.hasHydrated()) setHydrated(true);
    return unsub;
  }, [hydrated, store]);
  return hydrated;
}

/** Bottom sheet presentation for short tasks such as contacting a guest or blocking dates. */
function sheet(detents: number[]) {
  return {
    presentation: "formSheet" as const,
    sheetAllowedDetents: detents,
    sheetGrabberVisible: true,
    sheetCornerRadius: 28,
    contentStyle: { backgroundColor: colors.snow },
  };
}

export default function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    DMSans_400Regular,
    DMSans_500Medium,
    DMSans_600SemiBold,
  });
  const sessionHydrated = useSession((s) => s.hydrated);
  const onboarded = useSession((s) => s.onboarded);
  const draftHydrated = useHydrated(useOnboarding);
  const dataHydrated = useHydrated(useData);
  const ready = fontsLoaded && sessionHydrated && draftHydrated && dataHydrated;

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
          <Stack.Screen name="property/new" options={{ presentation: "modal" }} />
          <Stack.Screen name="paywall" options={{ presentation: "modal" }} />
          <Stack.Screen name="booking/new" options={{ presentation: "modal" }} />
          <Stack.Screen name="booking/payment" options={{ presentation: "modal" }} />
          <Stack.Screen name="booking/cancel" options={{ presentation: "modal" }} />
          <Stack.Screen name="booking/contact" options={sheet([0.55, 0.9])} />
          <Stack.Screen name="range/block" options={sheet([0.75, 0.95])} />
          <Stack.Screen name="range/rates" options={sheet([0.6, 0.9])} />
        </Stack.Protected>
      </Stack>
    </>
  );
}
