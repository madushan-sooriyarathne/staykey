import * as WebBrowser from "expo-web-browser";
import { Linking } from "react-native";

const SITE = "https://staykey.direct";

/** StayKey's pages on the web: support, the legal texts and account deletion. */
export const SITE_LINKS = {
  support: `${SITE}/support`,
  terms: `${SITE}/terms`,
  privacy: `${SITE}/privacy`,
  payoutTerms: `${SITE}/terms/payouts`,
  deleteAccount: `${SITE}/delete-account`,
} as const;

export const SUPPORT_EMAIL = "support@staykey.direct";
export const SUPPORT_WHATSAPP = "+94 71 688 0657";

/** Opens one of StayKey's pages in the in-app browser, or the system browser if that fails. */
export function openSite(url: string): void {
  WebBrowser.openBrowserAsync(url).catch(() => Linking.openURL(url));
}
