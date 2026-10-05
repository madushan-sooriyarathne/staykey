import { router } from "expo-router";
import { useState } from "react";
import { View } from "react-native";
import { Tag } from "@/components/brand";
import { I } from "@/components/icons";
import { Appear, Avatar, List, ListRow, SectionHeader, Segmented } from "@/components/kit";
import { Screen } from "@/components/ui";
import { useCan, useProperties } from "@/data/hooks";
import { plural } from "@/data/labels";
import { useData } from "@/data/store";
import { useOnboarding } from "@/features/onboarding/store";
import { addSampleGuesthouse } from "@/features/prototype/sample-guesthouse";
import { signOut } from "@/lib/auth";
import { haptics } from "@/lib/haptics";
import { PROTOTYPE } from "@/lib/prototype";
import { PLANS } from "@/lib/purchases";
import { type Role, useSession } from "@/lib/session";

const ROLE_LABEL: Record<Role, string> = {
  owner: "Owner",
  manager: "Manager",
  caretaker: "Caretaker",
};

export default function MoreScreen() {
  const { subscription, role, ownerName, setRole, reset: resetSession } = useSession();
  const account = useData((s) => s.account);
  const properties = useProperties();
  const unread = useData((s) => s.activity.filter((a) => !a.read).length);
  const [sampleState, setSampleState] = useState<"idle" | "adding" | string>("idle");
  const resetData = useData((s) => s.reset);
  const clearDraft = useOnboarding((s) => s.clear);
  const can = useCan();
  const plan =
    subscription.status === "active" ? PLANS.find((p) => p.id === subscription.plan) : undefined;
  const name = account.name || ownerName || "You";
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
        </List>
      </Appear>

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
            trailing={unread ? <Tag tone="ember" label={`${unread} new`} /> : undefined}
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
              trailing={<Tag tone="soft" label={plan ? plan.name : "Trial"} />}
              onPress={() => router.push("/subscription")}
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
                  resetData();
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
