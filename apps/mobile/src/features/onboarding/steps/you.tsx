import { colors, radius } from "@staykey/tokens";
import { SymbolView } from "expo-symbols";
import { useEffect, useRef, useState } from "react";
import { ActivityIndicator, Pressable, StyleSheet, Text, TextInput, View } from "react-native";
import Animated, {
  FadeIn,
  FadeOut,
  LinearTransition,
  useAnimatedStyle,
  useSharedValue,
  withSequence,
  withTiming,
} from "react-native-reanimated";
import { Checkbox, styles as c, Field } from "@/components/controls";
import { font } from "@/components/ui";
import { afterSignIn } from "@/features/auth/enter";
import { requestCode, useDevCode, verifyCode } from "@/lib/auth";
import { haptics } from "@/lib/haptics";
import { COUNTRIES, type Country, useOnboarding } from "../store";
import type { StepProps } from "../types";

/** Groups digits for display: Sri Lankan mobiles as 77 456 7890, others in threes. */
export function formatPhone(country: Country, digits: string): string {
  if (country.code === "LK") {
    return [digits.slice(0, 2), digits.slice(2, 5), digits.slice(5, 9)].filter(Boolean).join(" ");
  }
  return digits.replace(/(\d{3})(?=\d)/g, "$1 ");
}

export function isValidPhone(country: Country, digits: string): boolean {
  if (country.code === "LK") return /^7\d{8}$/.test(digits);
  return digits.length >= 6 && digits.length <= 14;
}

export function PhoneStep() {
  const { draft, update } = useOnboarding();
  const [picking, setPicking] = useState(false);

  return (
    <View style={{ gap: 16 }}>
      <Field
        testID="phone-input"
        autoFocus
        keyboardType="phone-pad"
        textContentType="telephoneNumber"
        autoComplete="tel"
        placeholder={draft.country.code === "LK" ? "77 123 4567" : "Phone number"}
        value={formatPhone(draft.country, draft.phone)}
        onChangeText={(t) => update({ phone: t.replace(/\D/g, "").replace(/^0/, "").slice(0, 14) })}
        prefix={
          <Pressable
            accessibilityRole="button"
            accessibilityLabel="Change country code"
            onPress={() => {
              haptics.select();
              setPicking((v) => !v);
            }}
            style={s.prefix}
          >
            <Text style={s.prefixText}>
              {draft.country.code} {draft.country.dial}
            </Text>
            <SymbolView
              name={{ ios: "chevron.down", android: "expand_more", web: "expand_more" }}
              tintColor={colors.steel}
              size={12}
            />
          </Pressable>
        }
      />

      {picking ? (
        <Animated.View
          entering={FadeIn.duration(180)}
          exiting={FadeOut.duration(120)}
          style={s.countries}
        >
          {COUNTRIES.map((country) => (
            <Pressable
              key={country.code}
              onPress={() => {
                haptics.select();
                update({ country, phone: "" });
                setPicking(false);
              }}
              style={s.countryRow}
            >
              <Text style={c.body}>{country.label}</Text>
              <Text style={c.muted}>{country.dial}</Text>
            </Pressable>
          ))}
        </Animated.View>
      ) : null}

      <Animated.View layout={LinearTransition.springify()}>
        <Checkbox
          checked={draft.whatsappAlerts}
          onChange={(whatsappAlerts) => update({ whatsappAlerts })}
          label="Also send booking alerts to this number on WhatsApp"
        />
      </Animated.View>
    </View>
  );
}

const CODE_LENGTH = 6;
const CODE_BOXES = ["c1", "c2", "c3", "c4", "c5", "c6"] as const;
const RESEND_SECONDS = 30;

export function VerifyStep({ next, goTo }: StepProps) {
  const { draft, update } = useOnboarding();
  const devCode = useDevCode((s) => s.code);
  const [code, setCode] = useState("");
  const [status, setStatus] = useState<"idle" | "checking" | "wrong" | "expired">("idle");
  const [message, setMessage] = useState<string | null>(null);
  const [seconds, setSeconds] = useState(RESEND_SECONDS);
  const input = useRef<TextInput>(null);
  const shake = useSharedValue(0);
  const shakeStyle = useAnimatedStyle(() => ({ transform: [{ translateX: shake.value }] }));
  const phone = `${draft.country.dial} ${formatPhone(draft.country, draft.phone)}`;
  const e164 = `${draft.country.dial}${draft.phone}`;

  useEffect(() => {
    if (seconds <= 0) return;
    const t = setTimeout(() => setSeconds((s) => s - 1), 1000);
    return () => clearTimeout(t);
  }, [seconds]);

  function fail(kind: "wrong" | "expired", text: string) {
    haptics.error();
    setStatus(kind);
    setMessage(text);
    // An expired or exhausted code can't be retried, so offer a new one straight away.
    if (kind === "expired") setSeconds(0);
    shake.value = withSequence(
      withTiming(-8, { duration: 50 }),
      withTiming(8, { duration: 50 }),
      withTiming(-6, { duration: 50 }),
      withTiming(6, { duration: 50 }),
      withTiming(0, { duration: 50 }),
    );
    setCode("");
    input.current?.focus();
  }

  async function submit(value: string) {
    setStatus("checking");
    setMessage(null);
    const result = await verifyCode(e164, value);
    if (!result.ok) {
      fail(result.reason === "expired" ? "expired" : "wrong", result.message);
      return;
    }

    const where = await afterSignIn(result.signIn);
    if (where.to === "error") {
      fail("expired", where.message);
      return;
    }
    haptics.success();
    // "app" flips the router guard to the tabs; setup carries on with the next step.
    if (where.to === "setup") {
      const name = result.signIn.user.name.trim();
      if (name && !draft.firstName) {
        const [first = "", ...rest] = name.split(" ");
        update({ firstName: first, lastName: rest.join(" ") });
      }
      next();
    }
  }

  async function resend() {
    haptics.select();
    setStatus("idle");
    setMessage(null);
    const result = await requestCode(e164);
    if (result.ok) {
      setSeconds(result.resendAfter);
    } else {
      setMessage(result.message);
      setSeconds(result.retryAfter ?? RESEND_SECONDS);
    }
  }

  return (
    <View style={{ gap: 16 }}>
      <Text style={c.muted}>
        Sent to {phone}.{" "}
        <Text style={s.inlineLink} onPress={() => goTo("phone")}>
          Change number
        </Text>
      </Text>

      <Pressable onPress={() => input.current?.focus()}>
        <Animated.View style={[s.otp, shakeStyle]}>
          {CODE_BOXES.map((box, i) => {
            const digit = code[i];
            const focused = i === code.length && status !== "checking";
            return (
              <View
                key={box}
                style={[s.box, focused && s.boxFocused, status === "wrong" && s.boxWrong]}
              >
                {digit ? (
                  <Animated.Text entering={FadeIn.duration(120)} style={s.digit}>
                    {digit}
                  </Animated.Text>
                ) : focused ? (
                  <View style={s.caret} />
                ) : null}
              </View>
            );
          })}
        </Animated.View>
      </Pressable>

      <TextInput
        ref={input}
        testID="otp-input"
        value={code}
        autoFocus
        keyboardType="number-pad"
        textContentType="oneTimeCode"
        autoComplete="sms-otp"
        maxLength={CODE_LENGTH}
        caretHidden
        style={s.hiddenInput}
        onChangeText={(t) => {
          const value = t.replace(/\D/g, "").slice(0, CODE_LENGTH);
          if (status === "wrong") {
            setStatus("idle");
            setMessage(null);
          }
          if (value.length > code.length) haptics.select();
          setCode(value);
          if (value.length === CODE_LENGTH) submit(value);
        }}
      />

      <View style={s.row}>
        {status === "checking" ? (
          <Animated.View entering={FadeIn} style={s.row}>
            <ActivityIndicator size="small" color={colors.steel} />
            <Text style={c.muted}>Checking code</Text>
          </Animated.View>
        ) : status === "wrong" && message ? (
          <Animated.Text entering={FadeIn} style={c.error}>
            {message}
          </Animated.Text>
        ) : seconds > 0 ? (
          <Text style={c.muted}>
            {message ? <Text style={c.error}>{message} </Text> : null}
            Resend code in {formatSeconds(seconds)}
          </Text>
        ) : (
          <Text style={c.muted}>
            {message ? <Text style={c.error}>{message} </Text> : null}
            <Text style={s.inlineLink} onPress={resend}>
              {status === "expired" ? "Send a new code" : "Resend code"}
            </Text>
          </Text>
        )}
      </View>

      {devCode ? (
        <Text testID="dev-code" style={c.muted}>
          Development server: your code is {devCode}.
        </Text>
      ) : null}
    </View>
  );
}

/** 0:30, or 4:05 when a limit means a longer wait. */
function formatSeconds(total: number): string {
  return `${Math.floor(total / 60)}:${String(total % 60).padStart(2, "0")}`;
}

export function NameStep() {
  const { draft, update } = useOnboarding();
  const last = useRef<TextInput>(null);
  return (
    <View style={{ gap: 14 }}>
      <Field
        testID="first-name"
        label="First name"
        autoFocus
        autoCapitalize="words"
        textContentType="givenName"
        autoComplete="given-name"
        returnKeyType="next"
        value={draft.firstName}
        onChangeText={(firstName) => update({ firstName })}
        onSubmitEditing={() => last.current?.focus()}
      />
      <Field
        ref={last}
        label="Last name"
        autoCapitalize="words"
        textContentType="familyName"
        autoComplete="family-name"
        value={draft.lastName}
        onChangeText={(lastName) => update({ lastName })}
      />
    </View>
  );
}

const s = StyleSheet.create({
  prefix: {
    flexDirection: "row",
    alignItems: "center",
    gap: 6,
    paddingRight: 10,
    borderRightWidth: 1,
    borderRightColor: colors.cloud,
    height: 28,
  },
  prefixText: { fontFamily: font.medium, fontSize: 15, color: colors.obsidian },
  countries: {
    backgroundColor: colors.snow,
    borderWidth: 1,
    borderColor: colors.cloud,
    borderRadius: 20,
    paddingHorizontal: 16,
    paddingVertical: 4,
  },
  countryRow: { flexDirection: "row", justifyContent: "space-between", paddingVertical: 12 },
  inlineLink: { fontFamily: font.medium, color: colors.obsidian, textDecorationLine: "underline" },
  otp: { flexDirection: "row", gap: 8 },
  box: {
    flex: 1,
    height: 58,
    borderRadius: radius.input,
    borderWidth: 1,
    borderColor: colors.cloud,
    backgroundColor: colors.snow,
    alignItems: "center",
    justifyContent: "center",
  },
  boxFocused: { borderColor: colors.ember, borderWidth: 1.5 },
  boxWrong: { borderColor: colors.emberInk },
  digit: { fontFamily: font.semibold, fontSize: 24, color: colors.obsidian },
  caret: { width: 2, height: 24, borderRadius: 1, backgroundColor: colors.ember },
  hiddenInput: { position: "absolute", width: 1, height: 1, opacity: 0 },
  row: { flexDirection: "row", alignItems: "center", gap: 8, minHeight: 22 },
});
