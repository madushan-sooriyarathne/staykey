import { useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { Tag } from "@/components/brand";
import { Field, OptionRow, Pill, Stepper } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, IconButton, List, ListRow, SectionHeader } from "@/components/kit";
import { POLICY_TEXT } from "@/data/pricing";
import type { Policy } from "@/data/types";
import { SettingsPage, useSettings } from "@/features/property/settings";

const KEYS = ["policy", "depositPercent", "balanceDueDays", "houseRules"] as const;

/** Cancellation policy, deposit and balance rules, and house rules guests accept at checkout. */
export default function Policies() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const { draft, set, dirty, save, status, property } = useSettings(id, KEYS);
  const [rule, setRule] = useState("");
  if (!draft || !property)
    return (
      <SettingsPage title="Policies" missing>
        {null}
      </SettingsPage>
    );

  return (
    <SettingsPage title="Policies" dirty={dirty} onSave={save} status={status}>
      <SectionHeader title="Cancellation" />
      {(Object.keys(POLICY_TEXT) as Policy[]).map((p) => (
        <OptionRow
          key={p}
          title={POLICY_TEXT[p].label}
          description={POLICY_TEXT[p].summary}
          selected={draft.policy === p}
          onPress={() => set({ policy: p })}
          badge={p === "moderate" ? <Tag tone="spark" label="Recommended" /> : undefined}
        />
      ))}
      <SectionHeader title="Deposit and balance" />
      <Card>
        <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8, paddingBottom: 6 }}>
          {[0, 30, 50, 100].map((d) => (
            <Pill
              key={d}
              label={d === 0 ? "No deposit" : d === 100 ? "Full amount" : `${d}% deposit`}
              selected={draft.depositPercent === d}
              onPress={() => set({ depositPercent: d })}
            />
          ))}
        </View>
        {draft.depositPercent > 0 && draft.depositPercent < 100 ? (
          <Stepper
            label="Balance due before arrival"
            hint="Days"
            value={draft.balanceDueDays}
            min={0}
            max={60}
            onChange={(balanceDueDays) => set({ balanceDueDays })}
          />
        ) : null}
      </Card>
      <List title="House rules">
        {draft.houseRules.map((r) => (
          <ListRow
            key={r}
            title={r}
            chevron={false}
            trailing={
              <IconButton
                icon={I.close}
                label={`Remove ${r}`}
                onPress={() => set({ houseRules: draft.houseRules.filter((x) => x !== r) })}
              />
            }
          />
        ))}
      </List>
      <Field
        label="Add a house rule"
        value={rule}
        onChangeText={setRule}
        placeholder="Quiet hours 10:00 PM to 7:00 AM"
        returnKeyType="done"
        onSubmitEditing={() => {
          if (rule.trim()) set({ houseRules: [...draft.houseRules, rule.trim()] });
          setRule("");
        }}
        suffix={
          rule.trim() ? (
            <IconButton
              icon={I.plus}
              label="Add rule"
              onPress={() => {
                set({ houseRules: [...draft.houseRules, rule.trim()] });
                setRule("");
              }}
            />
          ) : undefined
        }
      />
    </SettingsPage>
  );
}
