import { colors, radius } from "@staykey/tokens";
import { router } from "expo-router";
import { type ReactNode, useEffect, useMemo, useState } from "react";
import { StyleSheet, Text, View } from "react-native";
import Animated, { Easing, FadeInDown, FadeOut, LinearTransition } from "react-native-reanimated";
import { Button, Field, TextLink } from "@/components/controls";
import { I } from "@/components/icons";
import { EmptyState, Page } from "@/components/kit";
import { font } from "@/components/ui";
import { useProperty } from "@/data/hooks";
import { useData } from "@/data/store";
import type { Currency, PropertyConfig } from "@/data/types";
import { haptics } from "@/lib/haptics";

const ease = Easing.out(Easing.cubic);

type Picked<K extends keyof PropertyConfig> = Pick<PropertyConfig, K>;

/**
 * Local draft of some property settings with a dirty flag. Save writes the draft back and
 * returns to the overview, so every settings screen behaves the same way.
 */
export function useSettings<K extends keyof PropertyConfig>(id: string, keys: readonly K[]) {
  const property = useProperty(id);
  const update = useData((s) => s.updateProperty);
  // biome-ignore lint/correctness/useExhaustiveDependencies: keys are static per screen
  const initial = useMemo(
    () => (property ? (Object.fromEntries(keys.map((k) => [k, property[k]])) as Picked<K>) : null),
    [property],
  );
  const [draft, setDraft] = useState<Picked<K> | null>(initial);

  useEffect(() => {
    if (!draft && initial) setDraft(initial);
  }, [draft, initial]);

  const dirty = !!draft && JSON.stringify(draft) !== JSON.stringify(initial);
  return {
    property,
    draft,
    dirty,
    set: (patch: Partial<Picked<K>>) => setDraft((d) => (d ? { ...d, ...patch } : d)),
    save: () => {
      if (!draft) return;
      haptics.success();
      update(id, draft as Partial<PropertyConfig>);
      router.back();
    },
  };
}

export function SettingsPage({
  title,
  dirty,
  onSave,
  children,
  footer,
  missing,
}: {
  title: string;
  dirty?: boolean;
  onSave?: () => void;
  children: ReactNode;
  footer?: ReactNode;
  missing?: boolean;
}) {
  if (missing) {
    return (
      <Page title={title}>
        <EmptyState icon={I.house} title="Property not found" />
      </Page>
    );
  }
  return (
    <Page
      title={title}
      action={
        onSave
          ? { label: "Save", onPress: onSave, disabled: !dirty, testID: "settings-save" }
          : undefined
      }
      footer={footer}
    >
      {children}
    </Page>
  );
}

/** Expanding card for adding or editing one item in a list, with Cancel and Save. */
export function InlineEditor({
  title,
  children,
  onCancel,
  onSave,
  onDelete,
  saveDisabled,
  saveLabel = "Save",
}: {
  title: string;
  children: ReactNode;
  onCancel: () => void;
  onSave: () => void;
  onDelete?: () => void;
  saveDisabled?: boolean;
  saveLabel?: string;
}) {
  return (
    <Animated.View
      entering={FadeInDown.duration(240).easing(ease)}
      exiting={FadeOut.duration(140)}
      layout={LinearTransition.duration(220)}
      style={s.editor}
    >
      <Text style={s.editorTitle}>{title}</Text>
      {children}
      <View style={s.editorActions}>
        <View style={{ flex: 1 }}>
          <Button compact variant="ghost" title="Cancel" onPress={onCancel} />
        </View>
        <View style={{ flex: 1 }}>
          <Button
            compact
            title={saveLabel}
            disabled={saveDisabled}
            onPress={onSave}
            testID="editor-save"
          />
        </View>
      </View>
      {onDelete ? <TextLink title="Remove" onPress={onDelete} /> : null}
    </Animated.View>
  );
}

/** A money input that edits minor units through a major-unit text field. */
export function MoneyField({
  label,
  value,
  onChange,
  currency,
  placeholder,
  testID,
}: {
  label?: string;
  value: number;
  onChange: (minor: number) => void;
  currency: Currency;
  placeholder?: string;
  testID?: string;
}) {
  const [text, setText] = useState(value ? String(value / 100) : "");
  return (
    <Field
      testID={testID}
      label={label}
      value={text}
      placeholder={placeholder ?? "0"}
      keyboardType="decimal-pad"
      onChangeText={(t) => {
        const clean = t.replace(/[^\d.]/g, "");
        setText(clean);
        onChange(Math.round(Number(clean || 0) * 100));
      }}
      prefix={<Text style={s.prefix}>{currency === "USD" ? "$" : "LKR"}</Text>}
    />
  );
}

export const TIMES_IN = ["12:00", "13:00", "14:00", "15:00", "16:00"];
export const TIMES_OUT = ["10:00", "11:00", "12:00"];

export function DashedButton({
  title,
  onPress,
  testID,
}: {
  title: string;
  onPress: () => void;
  testID?: string;
}) {
  return (
    <Animated.View layout={LinearTransition.duration(200)}>
      <Text
        testID={testID}
        accessibilityRole="button"
        onPress={() => {
          haptics.select();
          onPress();
        }}
        style={s.dashed}
      >
        {title}
      </Text>
    </Animated.View>
  );
}

const s = StyleSheet.create({
  editor: {
    backgroundColor: colors.snow,
    borderRadius: radius.card,
    borderWidth: 1.5,
    borderColor: colors.obsidian,
    padding: 16,
    gap: 12,
  },
  editorTitle: { fontFamily: font.semibold, fontSize: 16, color: colors.obsidian },
  editorActions: { flexDirection: "row", gap: 8, paddingTop: 4 },
  prefix: { fontFamily: font.medium, fontSize: 16, color: colors.steel },
  dashed: {
    textAlign: "center",
    fontFamily: font.medium,
    fontSize: 15,
    color: colors.iron,
    borderWidth: 1.5,
    borderStyle: "dashed",
    borderColor: colors.mist,
    borderRadius: radius.button,
    paddingVertical: 15,
    overflow: "hidden",
  },
});
