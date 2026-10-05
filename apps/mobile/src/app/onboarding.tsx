import { colors } from "@staykey/tokens";
import { router, useFocusEffect } from "expo-router";
import { SymbolView } from "expo-symbols";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  BackHandler,
  KeyboardAvoidingView,
  Platform,
  Pressable,
  ScrollView,
  StyleSheet,
  Text,
  View,
} from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  useReducedMotion,
  useSharedValue,
} from "react-native-reanimated";
import { SafeAreaView } from "react-native-safe-area-context";
import { Button, InfoNote, TextLink } from "@/components/controls";
import { font } from "@/components/ui";
import { toPropertyConfig } from "@/data/from-api";
import { useData } from "@/data/store";
import { progressFor, publish, STAGES, stepsFor } from "@/features/onboarding/flow";
import { StageHeader } from "@/features/onboarding/stage-header";
import { formatPhone } from "@/features/onboarding/steps/you";
import { baseRateMinor, OTA_CHANNELS, useOnboarding } from "@/features/onboarding/store";
import type { StepId } from "@/features/onboarding/types";
import { requestCode } from "@/lib/auth";
import { haptics } from "@/lib/haptics";
import { stepEntering, stepExiting, webStepEntering, webStepExiting } from "@/lib/motion";
import { PROTOTYPE } from "@/lib/prototype";
import { useSession } from "@/lib/session";

const OTA_LABELS: Record<string, string> = {
  airbnb: "Airbnb",
  booking: "Booking.com",
  agoda: "Agoda",
  expedia: "Expedia",
};

/** Back to Welcome. Progress is already saved, so Welcome offers "Continue setup". */
function leave() {
  if (router.canGoBack()) router.back();
  else router.replace("/welcome");
}

/**
 * Onboarding steps 2 to 12 in one screen. The header and footer stay put while each step slides
 * in from the direction of travel, so moving forward and back reads as one continuous flow.
 */
export default function Onboarding() {
  const { draft, stepId, setStep, update } = useOnboarding();
  const publishSession = useSession((s) => s.publish);

  // The step list only changes when the booking type does.
  // biome-ignore lint/correctness/useExhaustiveDependencies: recompute on booking type only
  const steps = useMemo(() => stepsFor(draft), [draft.bookingType]);
  const index = Math.max(
    0,
    steps.findIndex((s) => s.id === stepId),
  );
  const step = steps[index] ?? steps[0];

  // Direction lives in a shared value so the outgoing step's exit, which was configured on its
  // last render, still reads the new direction when it runs on the UI thread.
  const direction = useSharedValue(1);
  const directionRef = useRef<1 | -1>(1);
  const reduceMotion = useReducedMotion();
  const nativeEntering = useMemo(
    () => stepEntering(direction, reduceMotion),
    [direction, reduceMotion],
  );
  const nativeExiting = useMemo(
    () => stepExiting(direction, reduceMotion),
    [direction, reduceMotion],
  );
  const isWeb = Platform.OS === "web";

  const [busy, setBusy] = useState(false);
  const [fieldError, setFieldError] = useState<string | null>(null);
  const [notice, setNotice] = useState<string | null>(null);

  useEffect(() => {
    if (!stepId && steps[0]) setStep(steps[0].id);
  }, [stepId, steps, setStep]);

  const moveTo = useCallback(
    (to: number, dir: 1 | -1) => {
      const target = steps[to];
      if (!target) return;
      direction.value = dir;
      directionRef.current = dir;
      setFieldError(null);
      setNotice(null);
      setStep(target.id);
    },
    [direction, setStep, steps],
  );

  const next = useCallback(() => {
    if (index >= steps.length - 1) return;
    haptics.step();
    moveTo(index + 1, 1);
  }, [index, moveTo, steps.length]);

  const back = useCallback(() => {
    if (busy) return true;
    if (index > 0) {
      haptics.back();
      moveTo(index - 1, -1);
    } else {
      haptics.back();
      leave();
    }
    return true;
  }, [busy, index, moveTo]);

  const goTo = useCallback(
    (id: StepId) => {
      const to = steps.findIndex((s) => s.id === id);
      if (to < 0 || to === index) return;
      moveTo(to, to < index ? -1 : 1);
    },
    [index, moveTo, steps],
  );

  // Android hardware back walks back through the steps.
  useFocusEffect(
    useCallback(() => {
      if (Platform.OS !== "android") return;
      const sub = BackHandler.addEventListener("hardwareBackPress", back);
      return () => sub.remove();
    }, [back]),
  );

  async function onContinue() {
    if (!step?.canContinue(draft)) return;

    if (step.id === "phone") {
      setBusy(true);
      const sent = await requestCode(`${draft.country.dial}${draft.phone}`);
      setBusy(false);
      if (!sent.ok) {
        haptics.error();
        setNotice(sent.message);
        return;
      }
      next();
      return;
    }

    if (step.id === "review") {
      setBusy(true);
      const result = await publish(draft);
      setBusy(false);
      if (result.ok) {
        haptics.success();
        useData.getState().startLocal(
          toPropertyConfig(result.property),
          {
            name: `${draft.firstName} ${draft.lastName}`.trim(),
            phone: `${draft.country.dial} ${formatPhone(draft.country, draft.phone)}`,
          },
          PROTOTYPE,
        );
        publishSession({
          ownerName: draft.firstName.trim(),
          propertyName: result.property.name,
          slug: result.property.slug,
          bookingPageUrl: result.property.bookingPageUrl,
          setup: {
            units:
              draft.bookingType === "entire" ? 1 : draft.rooms.reduce((n, r) => n + r.count, 0),
            currency: draft.currency,
            nightlyRate: baseRateMinor(draft),
            photoCount: draft.photos.length,
            otas: draft.channels
              .filter((c) => OTA_CHANNELS.includes(c))
              .map((c) => OTA_LABELS[c] ?? c),
          },
        });
        router.replace("/live");
      } else {
        haptics.error();
        // The session ended while setting up: confirm the number again, then publish.
        if (result.reauth) goTo("phone");
        if (result.field === "slug") setFieldError(result.message);
        else setNotice(result.message);
      }
      return;
    }

    next();
  }

  if (!step) return null;
  const stageIndex = STAGES.findIndex((s) => s.id === step.stage);
  const canContinue = step.canContinue(draft);
  const StepBody = step.Component;

  return (
    <SafeAreaView edges={["top", "bottom"]} style={s.screen}>
      <KeyboardAvoidingView
        behavior={Platform.OS === "ios" ? "padding" : undefined}
        style={{ flex: 1 }}
      >
        <View style={s.topBar}>
          <Pressable
            testID="onboarding-back"
            accessibilityRole="button"
            accessibilityLabel="Back"
            hitSlop={8}
            onPress={back}
            style={s.iconButton}
          >
            <SymbolView
              name={{ ios: "chevron.left", android: "arrow_back", web: "arrow_back" }}
              tintColor={colors.iron}
              size={18}
            />
          </Pressable>
          {stageIndex > 0 ? (
            <Animated.View entering={FadeIn.duration(200)} exiting={FadeOut.duration(120)}>
              <Pressable
                hitSlop={8}
                onPress={() => {
                  haptics.select();
                  leave();
                }}
              >
                <Text style={s.finishLater}>Finish later</Text>
              </Pressable>
            </Animated.View>
          ) : null}
        </View>

        <StageHeader progress={progressFor(steps, index)} stageIndex={stageIndex} />

        <View style={s.stage}>
          <Animated.View
            key={step.id}
            entering={isWeb ? webStepEntering(directionRef.current, reduceMotion) : nativeEntering}
            exiting={isWeb ? webStepExiting : nativeExiting}
            style={StyleSheet.absoluteFill}
          >
            <ScrollView
              contentContainerStyle={s.content}
              keyboardShouldPersistTaps="handled"
              showsVerticalScrollIndicator={false}
            >
              <View style={{ gap: 6 }}>
                <Text style={s.title}>{step.title(draft)}</Text>
                {step.subtitle ? <Text style={s.subtitle}>{step.subtitle(draft)}</Text> : null}
              </View>
              <StepBody
                next={next}
                goTo={goTo}
                error={fieldError}
                clearError={() => setFieldError(null)}
              />
              {notice ? (
                <InfoNote
                  icon={{ ios: "wifi.exclamationmark", android: "wifi_off", web: "wifi_off" }}
                >
                  {notice}
                </InfoNote>
              ) : null}
            </ScrollView>
          </Animated.View>
        </View>

        {step.cta ? (
          <View style={s.footer}>
            <Button
              testID="onboarding-continue"
              title={step.cta(draft)}
              disabled={!canContinue}
              loading={busy}
              onPress={onContinue}
              icon={
                step.id === "review"
                  ? { ios: "globe", android: "public", web: "public" }
                  : undefined
              }
            />
            {step.secondary && !canContinue ? (
              <Animated.View
                entering={FadeIn.duration(180)}
                exiting={FadeOut.duration(120)}
                style={{ paddingTop: 14 }}
              >
                <TextLink
                  title={step.secondary.label}
                  onPress={() => {
                    if (step.secondary) update(step.secondary.patch);
                    next();
                  }}
                />
              </Animated.View>
            ) : null}
          </View>
        ) : null}
      </KeyboardAvoidingView>
    </SafeAreaView>
  );
}

const s = StyleSheet.create({
  screen: { flex: 1, backgroundColor: colors.paper },
  topBar: {
    height: 52,
    flexDirection: "row",
    alignItems: "center",
    justifyContent: "space-between",
    paddingHorizontal: 16,
  },
  iconButton: {
    width: 40,
    height: 40,
    borderRadius: 20,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    alignItems: "center",
    justifyContent: "center",
  },
  finishLater: { fontFamily: font.medium, fontSize: 15, color: colors.iron },
  stage: { flex: 1, overflow: "hidden" },
  content: { paddingHorizontal: 20, paddingTop: 24, paddingBottom: 28, gap: 20 },
  title: { fontFamily: font.semibold, fontSize: 28, lineHeight: 34, color: colors.obsidian },
  subtitle: { fontFamily: font.regular, fontSize: 15, lineHeight: 21, color: colors.steel },
  footer: {
    paddingHorizontal: 16,
    paddingTop: 10,
    paddingBottom: Platform.OS === "android" ? 16 : 6,
  },
});
