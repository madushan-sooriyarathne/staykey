import { colors } from "@staykey/tokens";
import { StyleSheet, Text } from "react-native";
import { font } from "@/components/ui";
import { TEMPLATE_VARIABLES } from "@/data/defaults";

const LABEL = Object.fromEntries(TEMPLATE_VARIABLES.map((v) => [v.key, v.label.toLowerCase()]));

/** Renders a template with each {field} shown as a highlighted chip. */
export function TemplateText({ body }: { body: string }) {
  const parts = body.split(/(\{\w+\})/g);
  return (
    <Text style={s.body}>
      {parts.map((part, i) => {
        const m = part.match(/^\{(\w+)\}$/);
        return m ? (
          // biome-ignore lint/suspicious/noArrayIndexKey: parts are positional
          <Text key={i} style={s.var}>
            {` ${LABEL[m[1] ?? ""] ?? m[1]} `}
          </Text>
        ) : (
          part
        );
      })}
    </Text>
  );
}

const s = StyleSheet.create({
  body: {
    fontFamily: font.regular,
    fontSize: 15,
    lineHeight: 24,
    color: colors.graphite,
    paddingBottom: 12,
  },
  var: { fontFamily: font.medium, color: colors.emberInk, backgroundColor: colors.emberTint },
});
