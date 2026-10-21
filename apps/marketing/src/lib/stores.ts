import "server-only";
import type { Store } from "./store-for";

/** The App Store and Google Play listing links, or null until a listing exists. */
export function storeLinks(): Record<Store, string | null> {
  return {
    ios: process.env.STAYKEY_APP_STORE_URL || null,
    android: process.env.STAYKEY_PLAY_STORE_URL || null,
  };
}

/** The App Store app id for Apple's Smart App Banner, or null. */
export function appStoreId(): string | null {
  return process.env.STAYKEY_APP_STORE_ID || null;
}
