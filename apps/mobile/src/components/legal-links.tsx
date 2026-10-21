import { View } from "react-native";
import { openSite, SITE_LINKS } from "@/lib/site";
import { TextLink } from "./controls";

/** Terms of use and privacy policy, which the stores want beside every purchase. */
export function LegalLinks() {
  return (
    <View style={{ flexDirection: "row", justifyContent: "center", gap: 20, flexWrap: "wrap" }}>
      <TextLink
        testID="link-terms"
        title="Terms of use"
        onPress={() => openSite(SITE_LINKS.terms)}
      />
      <TextLink
        testID="link-privacy"
        title="Privacy policy"
        onPress={() => openSite(SITE_LINKS.privacy)}
      />
    </View>
  );
}
