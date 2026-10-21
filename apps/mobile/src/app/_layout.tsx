import {
  DMSans_400Regular,
  DMSans_500Medium,
  DMSans_600SemiBold,
  useFonts,
} from "@expo-google-fonts/dm-sans";
import AsyncStorage from "@react-native-async-storage/async-storage";
import { colors } from "@staykey/tokens";
import { PersistQueryClientProvider } from "@tanstack/react-query-persist-client";
import { Stack } from "expo-router";
import * as SplashScreen from "expo-splash-screen";
import { StatusBar } from "expo-status-bar";
import { useEffect, useState } from "react";
import { persistOptions, queryClient } from "@/api/query-client";
import { SessionCheck } from "@/features/auth/session-check";
import { useOnboarding } from "@/features/onboarding/store";
import { usePushAlerts } from "@/lib/push";
import { wrap } from "@/lib/sentry";
import { useSession } from "@/lib/session";

export { ErrorBoundary } from "expo-router";

SplashScreen.preventAutoHideAsync();

// Activity, team and templates lived in this on-device store until they moved to the API.
AsyncStorage.removeItem("staykey.data").catch(() => {});

type Persisted = {
  persist: { hasHydrated: () => boolean; onFinishHydration: (fn: () => void) => () => void };
};

/**
 * True once a persisted store has loaded from storage. The root waits for the onboarding draft,
 * so Welcome knows whether to resume.
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

// Wrapped so render errors and crashes reach Sentry in production builds.
export default wrap(RootLayout);

function RootLayout() {
  const [fontsLoaded, fontError] = useFonts({
    DMSans_400Regular,
    DMSans_500Medium,
    DMSans_600SemiBold,
  });
  const sessionHydrated = useSession((s) => s.hydrated);
  const onboarded = useSession((s) => s.onboarded);
  const draftHydrated = useHydrated(useOnboarding);
  // Saved server data (properties and settings) restores before the first screen draws.
  const [cacheRestored, setCacheRestored] = useState(false);
  const ready = fontsLoaded && sessionHydrated && draftHydrated && cacheRestored;

  useEffect(() => {
    if (fontError) throw fontError;
  }, [fontError]);

  useEffect(() => {
    if (ready) SplashScreen.hideAsync();
  }, [ready]);

  return (
    <PersistQueryClientProvider
      client={queryClient}
      persistOptions={persistOptions}
      onSuccess={() => setCacheRestored(true)}
      onError={() => setCacheRestored(true)}
    >
      {ready ? <App onboarded={onboarded} /> : null}
    </PersistQueryClientProvider>
  );
}

function App({ onboarded }: { onboarded: boolean }) {
  usePushAlerts(onboarded);
  return (
    <>
      <SessionCheck />
      <StatusBar style="dark" />
      <Stack
        screenOptions={{ contentStyle: { backgroundColor: colors.paper }, headerShown: false }}
      >
        {/* Signed out or still setting up. Finishing onboarding flips the guard and lands on Today. */}
        <Stack.Protected guard={!onboarded}>
          <Stack.Screen name="welcome" />
          <Stack.Screen name="onboarding" options={{ gestureEnabled: false }} />
          <Stack.Screen name="live" options={{ animation: "fade", gestureEnabled: false }} />
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

        {/*
          Invite links open here whether or not someone is signed in on this phone. It comes after
          both groups so that flipping a guard never lands on it as the first screen available.
        */}
        <Stack.Screen name="join" />
      </Stack>
    </>
  );
}
