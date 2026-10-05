import { useState } from "react";
import { Text, View } from "react-native";
import { Tag } from "@/components/brand";
import { Field, Pill } from "@/components/controls";
import { Appear, Avatar, Card, List, ListRow, Page, SectionHeader, ui } from "@/components/kit";
import { useProperties } from "@/data/hooks";
import { useData } from "@/data/store";
import type { TeamMember } from "@/data/types";
import { InlineEditor } from "@/features/property/settings";
import { openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

const ROLE: Record<TeamMember["role"], string> = {
  owner: "Owner",
  manager: "Manager",
  caretaker: "Caretaker",
};

/** Members, pending invites, and each person's role and property access. */
export default function Team() {
  const team = useData((s) => s.team);
  const properties = useProperties();
  const invite = useData((s) => s.invite);
  const updateMember = useData((s) => s.updateMember);
  const removeMember = useData((s) => s.removeMember);
  const [editing, setEditing] = useState<TeamMember | "new" | null>(null);

  const access = (m: TeamMember) =>
    m.propertyIds.length === 0
      ? "all properties"
      : m.propertyIds
          .map((id) => properties.find((p) => p.id === id)?.name)
          .filter(Boolean)
          .join(", ");

  return (
    <Page
      title="Team"
      action={{ label: "Invite", onPress: () => setEditing("new"), testID: "team-invite" }}
    >
      <Appear index={0}>
        <List>
          {team.map((m) =>
            editing !== "new" && editing?.id === m.id ? null : (
              <ListRow
                key={m.id}
                leading={<Avatar name={m.name} tone={m.role === "owner" ? "dark" : "neutral"} />}
                title={m.name || m.phone}
                subtitle={`${ROLE[m.role]}, ${access(m)}`}
                chevron={m.role !== "owner"}
                trailing={
                  m.role === "owner" ? (
                    <Tag tone="soft" label="You" />
                  ) : m.status === "invited" ? (
                    <Tag tone="dash" label="Invited" />
                  ) : undefined
                }
                onPress={m.role === "owner" ? undefined : () => setEditing(m)}
              />
            ),
          )}
        </List>
      </Appear>
      {editing ? (
        <MemberEditor
          member={editing === "new" ? undefined : editing}
          properties={properties.map((p) => ({ id: p.id, name: p.name }))}
          onCancel={() => setEditing(null)}
          onSave={(m) => {
            haptics.success();
            if (editing === "new") {
              invite({ phone: m.phone, role: m.role, propertyIds: m.propertyIds });
              openWhatsApp(
                m.phone,
                `You're invited to help run ${properties[0]?.name ?? "our property"} on StayKey. Open this link to join: https://staykey.direct/join`,
              );
            } else updateMember(editing.id, m);
            setEditing(null);
          }}
          onRemove={
            editing === "new"
              ? undefined
              : () => {
                  haptics.warning();
                  removeMember(editing.id);
                  setEditing(null);
                }
          }
        />
      ) : null}
      <SectionHeader title="What each role can do" />
      <Appear index={1}>
        <Card>
          <Text style={ui.strong}>Manager</Text>
          <Text style={ui.muted}>
            Bookings, calendar, rates and guests. No billing or account settings.
          </Text>
          <View style={{ height: 12 }} />
          <Text style={ui.strong}>Caretaker</Text>
          <Text style={ui.muted}>
            Arrivals, departures and guest contact. No prices or payments.
          </Text>
        </Card>
      </Appear>
    </Page>
  );
}

function MemberEditor({
  member,
  properties,
  onCancel,
  onSave,
  onRemove,
}: {
  member?: TeamMember;
  properties: { id: string; name: string }[];
  onCancel: () => void;
  onSave: (m: Pick<TeamMember, "phone" | "role" | "propertyIds">) => void;
  onRemove?: () => void;
}) {
  const [phone, setPhone] = useState(member?.phone ?? "+94 ");
  const [role, setRole] = useState<TeamMember["role"]>(member?.role ?? "caretaker");
  const [ids, setIds] = useState<string[]>(member?.propertyIds ?? []);
  return (
    <InlineEditor
      title={member ? `Edit ${member.name || member.phone}` : "Invite someone"}
      saveLabel={member ? "Save" : "Send invite"}
      onCancel={onCancel}
      onSave={() => onSave({ phone: phone.trim(), role, propertyIds: ids })}
      onDelete={onRemove}
      saveDisabled={phone.replace(/\D/g, "").length < 9}
    >
      {member ? null : (
        <Field
          testID="invite-phone"
          label="Their WhatsApp number"
          value={phone}
          onChangeText={setPhone}
          keyboardType="phone-pad"
        />
      )}
      <SectionHeader title="Role" />
      <View style={{ flexDirection: "row", gap: 8 }}>
        <Pill label="Manager" selected={role === "manager"} onPress={() => setRole("manager")} />
        <Pill
          label="Caretaker"
          selected={role === "caretaker"}
          onPress={() => setRole("caretaker")}
        />
      </View>
      {properties.length > 1 ? (
        <>
          <SectionHeader title="Properties" />
          <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
            <Pill label="All properties" selected={ids.length === 0} onPress={() => setIds([])} />
            {properties.map((p) => (
              <Pill
                key={p.id}
                label={p.name}
                selected={ids.includes(p.id)}
                onPress={() =>
                  setIds((x) => (x.includes(p.id) ? x.filter((y) => y !== p.id) : [...x, p.id]))
                }
              />
            ))}
          </View>
        </>
      ) : null}
      {member ? null : (
        <Text style={ui.faint}>We'll open WhatsApp with an invite link they can tap to join.</Text>
      )}
    </InlineEditor>
  );
}
