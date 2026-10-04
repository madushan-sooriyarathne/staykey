import AsyncStorage from "@react-native-async-storage/async-storage";
import type { AuthTokens } from "@staykey/api-client";
import * as SecureStore from "expo-secure-store";
import { Platform } from "react-native";

/**
 * The signed-in session's tokens. On iOS and Android they live in the keychain or keystore; the
 * web preview has no secure store, so it falls back to AsyncStorage there. A copy is kept in
 * memory so requests don't read storage every time.
 */

const KEY = "staykey.tokens";
const secure = Platform.OS !== "web";
// Readable after the first unlock, so a refresh can run when the app wakes in the background.
const options: SecureStore.SecureStoreOptions = {
  keychainAccessible: SecureStore.AFTER_FIRST_UNLOCK,
};

let cache: AuthTokens | null | undefined;

export async function loadTokens(): Promise<AuthTokens | null> {
  if (cache !== undefined) return cache;
  try {
    const raw = secure
      ? await SecureStore.getItemAsync(KEY, options)
      : await AsyncStorage.getItem(KEY);
    cache = raw ? (JSON.parse(raw) as AuthTokens) : null;
  } catch {
    cache = null;
  }
  return cache;
}

export async function saveTokens(tokens: AuthTokens): Promise<void> {
  cache = tokens;
  const raw = JSON.stringify(tokens);
  if (secure) await SecureStore.setItemAsync(KEY, raw, options);
  else await AsyncStorage.setItem(KEY, raw);
}

export async function clearTokens(): Promise<void> {
  cache = null;
  if (secure) await SecureStore.deleteItemAsync(KEY, options);
  else await AsyncStorage.removeItem(KEY);
}

/** True when the access token expires within `withinMs`. */
export function expiresSoon(tokens: AuthTokens, withinMs = 30_000): boolean {
  return new Date(tokens.accessTokenExpiresAt).getTime() - Date.now() < withinMs;
}
