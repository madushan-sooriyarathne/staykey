import type { Subscription } from "@staykey/api-client";
import { router } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { useUnreadCount } from "@/api/activity";
import { useMe } from "@/api/me";
import { useSubscription } from "@/api/subscription";
import { Tag } from "@/components/brand";
import { I } from "@/components/icons";
import { Appear, Avatar, List, ListRow, Menu, SectionHeader, Segmented } from "@/components/kit";
import { Screen } from "@/components/ui";
import { useCan, useProperties } from "@/data/hooks";
import { plural } from "@/data/labels";
import { openAccount } from "@/features/auth/enter";
import { useOnboarding } from "@/features/onboarding/store";
import { addSampleGuesthouse } from "@/features/prototype/sample-guesthouse";
import { signOut } from "@/lib/auth";
import { haptics } from "@/lib/haptics";
import { PROTOTYPE } from "@/lib/prototype";
import { PLANS } from "@/lib/purchases";
import { type Role, useSession } from "@/lib/session";

/** The plan in a word or two, for the Subscription row. */
function planLabel(sub: Subscription): string {
  switch (sub.status) {
    case "active":
      return PLANS.find((p) => p.id === sub.plan)?.name ?? "Active";
    case "trial":
      return "Free";
    case "grace":
      return "Choose a plan";
    default:
      return "Paused";
  }
}

const ROLE_LABEL: Record<Role, string> = {
  owner: "Owner",
  manager: "Manager",
  caretaker: "Caretaker",
};

export default function MoreScreen() {
  const { role, ownerName, accountId, setRole, reset: resetSession } = useSession();
  const subscription = useSubscription().data;
  const me = useMe().data;
  const properties = useProperties();
  const unread = useUnreadCount();
  const [sampleState, setSampleState] = useState<"idle" | "adding" | string>("idle");
  const [switching, setSwitching] = useState(false);
  const clearDraft = useOnboarding((s) => s.clear);
  const can = useCan();
  const name = me?.user.name || ownerName || "You";
  const accounts = me?.accounts ?? [];
  const current = accounts.find((a) => a.id === accountId);
  const hasGuesthouse = properties.some((p) => p.slug.startsWith("coralbay"));

  return (
    <Screen title="More">
      <Appear index={0}>
        <List>
          <ListRow
            testID="more-profile"
            leading={<Avatar name={name} size={44} tone="dark" />}
            title={name}
            subtitle={`${ROLE_LABEL[role]}, ${plural(properties.length, "property", "properties")}`}
            onPress={() => router.push("/profile")}
          />
          {accounts.length > 1 ? (
            <ListRow
              testID="more-switch-account"
              icon={I.house}
              title={current?.name ?? "Account"}
              subtitle="Switch account"
              onPress={() => setSwitching(true)}
            />
          ) : null}
        </List>
      </Appear>
      <Menu
        visible={switching}
        onClose={() => setSwitching(false)}
        items={accounts
          .filter((a) => a.id !== accountId)
          .map((a) => ({
            label: `${a.name} (${ROLE_LABEL[a.role].toLowerCase()})`,
            icon: I.house,
            onPress: async () => {
              // The display fallback ("You") isn't a name, so it never becomes one.
              const opened = await openAccount(a, me?.user.name ?? "");
              if (opened) haptics.success();
              else haptics.error();
            },
          }))}
      />

      <Appear index={1}>
        <List>
          {can("prices") ? (
            <ListRow
              testID="more-insights"
              icon={I.insights}
              title="Insights"
              onPress={() => router.push("/insights")}
            />
          ) : null}
          <ListRow
            testID="more-activity"
            icon={I.activity}
            title="Activity"
            trailing={unread > 0 ? <Tag tone="ember" label={`${unread} new`} /> : undefined}
            onPress={() => router.push("/activity")}
          />
          {can("team") ? (
            <ListRow
              testID="more-team"
              icon={I.team}
              title="Team"
              onPress={() => router.push("/team")}
            />
          ) : null}
        </List>
      </Appear>

      <Appear index={2}>
        <List>
          {can("settings") ? (
            <ListRow
              testID="more-templates"
              icon={I.template}
              title="Message templates"
              onPress={() => router.push("/templates")}
            />
          ) : null}
          <ListRow
            testID="more-notifications"
            icon={I.bell}
            title="Notification settings"
            onPress={() => router.push("/notifications")}
          />
        </List>
      </Appear>

      <Appear index={3}>
        <List>
          {can("billing") ? (
            <ListRow
              testID="more-subscription"
              icon={I.crown}
              title="Subscription"
              trailing={
                subscription ? <Tag tone="soft" label={planLabel(subscription)} /> : undefined
              }
              onPress={() => router.push("/subscription")}
            />
          ) : null}
          {can("billing") ? (
            <ListRow
              testID="more-card-payments"
              icon={I.card}
              title="Card payments"
              onPress={() => router.push("/card-payments")}
            />
          ) : null}
          <ListRow
            icon={I.person}
            title="Profile and account"
            onPress={() => router.push("/profile")}
          />
          <ListRow
            testID="more-help"
            icon={I.help}
            title="Help and support"
            onPress={() => router.push("/help")}
          />
        </List>
      </Appear>

      {PROTOTYPE ? (
        <Appear index={4}>
          <View style={{ gap: 8 }}>
            <SectionHeader title="Prototype" />
            <Segmented
              testID="role"
              options={[
                { id: "owner", label: "Owner" },
                { id: "manager", label: "Manager" },
                { id: "caretaker", label: "Caretaker" },
              ]}
              value={role}
              onChange={(r) => setRole(r)}
            />
            <List>
              {role === "owner" ? (
                <ListRow
                  icon={I.sparkles}
                  title="Preview first-booking paywall"
                  onPress={() =>
                    router.push({ pathname: "/paywall", params: { reason: "first-booking" } })
                  }
                />
              ) : null}
              {!hasGuesthouse ? (
                <ListRow
                  testID="more-sample-guesthouse"
                  icon={I.house}
                  title="Add a sample guesthouse"
                  subtitle={
                    sampleState === "adding"
                      ? "Publishing the sample"
                      : sampleState === "idle"
                        ? "Rooms, a whole-house unit and bookings"
                        : sampleState
                  }
                  onPress={async () => {
                    if (sampleState === "adding") return;
                    setSampleState("adding");
                    const error = await addSampleGuesthouse();
                    if (error) haptics.error();
                    else haptics.success();
                    setSampleState(error ?? "idle");
                  }}
                />
              ) : null}
              <ListRow
                testID="more-restart"
                icon={I.restart}
                title="Restart onboarding"
                onPress={() => {
                  haptics.warning();
                  signOut();
                  clearDraft();
                  resetSession();
                }}
              />
            </List>
          </View>
        </Appear>
      ) : null}
    </Screen>
  );
}
