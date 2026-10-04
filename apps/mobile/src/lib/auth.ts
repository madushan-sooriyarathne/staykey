import type { SignIn } from "@staykey/api-client";
import * as Device from "expo-device";
import { create } from "zustand";
import { authApi } from "@/api/client";
import { queryClient } from "@/api/query-client";
import { clearTokens, loadTokens, saveTokens } from "./tokens";

/**
 * Phone sign-in against the API: request a code, then verify it. A verified code stores the
 * session's tokens in the secure store; the caller decides where the app goes next.
 */

const OFFLINE = "Can't reach StayKey right now. Check your connection and try again.";

/**
 * Development servers return the code with the request so a phone can sign in without SMS.
 * The code step shows it in development builds.
 */
export const useDevCode = create<{ code: string | null }>(() => ({ code: null }));

export type CodeResult =
  | { ok: true; resendAfter: number }
  | { ok: false; message: string; retryAfter?: number };

export async function requestCode(phone: string): Promise<CodeResult> {
  try {
    const { data, error } = await authApi.POST("/v1/auth/otp", { body: { phone } });
    if (data) {
      useDevCode.setState({ code: data.devCode ?? null });
      return { ok: true, resendAfter: data.resendAfter };
    }
    return {
      ok: false,
      message: error?.message ?? "We couldn't send a code. Please try again.",
      retryAfter: error?.retryAfter,
    };
  } catch {
    return { ok: false, message: OFFLINE };
  }
}

export type VerifyResult =
  | { ok: true; signIn: SignIn }
  | {
      ok: false;
      /** wrong: try again. expired: the code can't be used any more, send a new one. */
      reason: "wrong" | "expired" | "invalid" | "offline";
      message: string;
    };

export async function verifyCode(phone: string, code: string): Promise<VerifyResult> {
  try {
    const { data, error } = await authApi.POST("/v1/auth/verify", {
      body: { phone, code, deviceName: deviceName() },
    });
    if (data) {
      await saveTokens(data.tokens);
      useDevCode.setState({ code: null });
      return { ok: true, signIn: data };
    }
    const reason =
      error?.code === "wrong_code"
        ? "wrong"
        : error?.code === "code_expired" || error?.code === "too_many_attempts"
          ? "expired"
          : "invalid";
    return { ok: false, reason, message: error?.message ?? "That code didn't work." };
  } catch {
    return { ok: false, reason: "offline", message: OFFLINE };
  }
}

/** Ends the session on the server and forgets the tokens and cached data on this device. */
export async function signOut(): Promise<void> {
  const tokens = await loadTokens();
  await clearTokens();
  queryClient.clear();
  if (!tokens) return;
  try {
    await authApi.POST("/v1/auth/logout", { body: { refreshToken: tokens.refreshToken } });
  } catch {
    // Offline: the refresh token is gone from this device and expires on its own.
  }
}

/** Whether this device holds a session. */
export async function hasSession(): Promise<boolean> {
  return (await loadTokens()) !== null;
}

function deviceName(): string | undefined {
  const name = Device.deviceName ?? Device.modelName;
  return name ? name.slice(0, 80) : undefined;
}
