import { useState } from "react";
import { Text, View } from "react-native";
import { messageFor } from "@/api/errors";
import { useInvite, useRemoveMember, useRevokeInvite, useTeam, useUpdateMember } from "@/api/team";
import { Tag } from "@/components/brand";
import { Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Avatar, Card, List, ListRow, Page, SectionHeader, ui } from "@/components/kit";
import { useCan, useProperties } from "@/data/hooks";
import { InlineEditor } from "@/features/property/settings";
import { openWhatsApp } from "@/lib/contact";
import { haptics } from "@/lib/haptics";
import { useSession } from "@/lib/session";

type Role = "owner" | "manager" | "caretaker";

/** A member or an open invite, as one row in the team list. */
type Person = {
  id: string;
  kind: "member" | "invite";
  name: string;
  phone: string;
  role: Role;
  /** Empty means every property. */
  propertyIds: string[];
  isMe: boolean;
};

const ROLE: Record<Role, string> = {
  owner: "Owner",
  manager: "Manager",
  caretaker: "Caretaker",
};

/** Members, pending invites, and each person's role and property access. */
export default function Team() {
  const team = useTeam();
  const userId = useSession((s) => s.userId);
  const properties = useProperties();
  const can = useCan();
  const invite = useInvite();
  const revoke = useRevokeInvite();
  const updateMember = useUpdateMember();
  const removeMember = useRemoveMember();
  const [editing, setEditing] = useState<Person | "new" | null>(null);
  const [error, setError] = useState<string | null>(null);
  const manages = can("team");

  const people: Person[] = [
    ...team.members.map((m) => ({
      id: m.id,
      kind: "member" as const,
      name: m.name,
      phone: m.phone,
      role: m.role,
      propertyIds: m.propertyIds,
      isMe: m.userId === userId,
    })),
    ...team.invites.map((i) => ({
      id: i.id,
      kind: "invite" as const,
      name: i.name,
      phone: i.phone,
      role: i.role,
      propertyIds: i.propertyIds,
      isMe: false,
    })),
  ];

  const access = (p: Person) =>
    p.propertyIds.length === 0
      ? "all properties"
      : p.propertyIds
          .map((id) => properties.find((x) => x.id === id)?.name)
          .filter(Boolean)
          .join(", ");

  const fail = (e: unknown) => {
    haptics.error();
    setError(messageFor(e));
  };

  /** Creates the invite, then opens WhatsApp with its link for the owner to send. */
  function sendInvite(
    m: Pick<Person, "phone" | "propertyIds"> & { role: "manager" | "caretaker" },
  ) {
    invite.mutate(
      { phone: m.phone, role: m.role, propertyIds: m.propertyIds },
      {
        onSuccess: ({ link }) => {
          haptics.success();
          setEditing(null);
          const place =
            m.propertyIds.length === 1
              ? (properties.find((p) => p.id === m.propertyIds[0])?.name ?? "our property")
              : (properties[0]?.name ?? "our property");
          openWhatsApp(
            m.phone,
            `You're invited to help run ${place} on StayKey. Open this link to join: ${link}`,
          );
        },
        onError: fail,
      },
    );
  }

  function save(
    p: Person,
    m: Pick<Person, "phone" | "propertyIds"> & { role: "manager" | "caretaker" },
  ) {
    if (p.kind === "invite") {
      sendInvite(m);
      return;
    }
    updateMember.mutate(
      { id: p.id, patch: { role: m.role, propertyIds: m.propertyIds } },
      {
        onSuccess: () => {
          haptics.success();
          setEditing(null);
        },
        onError: fail,
      },
    );
  }

  function remove(p: Person) {
    haptics.warning();
    const done = { onSuccess: () => setEditing(null), onError: fail };
    if (p.kind === "invite") revoke.mutate(p.id, done);
    else removeMember.mutate(p.id, done);
  }

  return (
    <Page
      title="Team"
      action={
        manages
          ? {
              label: "Invite",
              onPress: () => {
                setError(null);
                setEditing("new");
              },
              testID: "team-invite",
            }
          : undefined
      }
    >
      {error ? (
        <InfoNote icon={I.warning} testID="team-error">
          {error}
        </InfoNote>
      ) : null}
      <Appear index={0}>
        <List>
          {people.map((p) =>
            editing !== "new" && editing?.id === p.id ? null : (
              <ListRow
                key={p.id}
                leading={<Avatar name={p.name} tone={p.role === "owner" ? "dark" : "neutral"} />}
                title={p.name || p.phone}
                subtitle={`${ROLE[p.role]}, ${access(p)}`}
                chevron={manages && p.role !== "owner"}
                trailing={
                  p.isMe ? (
                    <Tag tone="soft" label="You" />
                  ) : p.kind === "invite" ? (
                    <Tag tone="dash" label="Invited" />
                  ) : undefined
                }
                onPress={
                  manages && p.role !== "owner"
                    ? () => {
                        setError(null);
                        setEditing(p);
                      }
                    : undefined
                }
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
          onSave={(m) => (editing === "new" ? sendInvite(m) : save(editing, m))}
          onRemove={editing === "new" ? undefined : () => remove(editing)}
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
  member?: Person;
  properties: { id: string; name: string }[];
  onCancel: () => void;
  onSave: (m: Pick<Person, "phone" | "propertyIds"> & { role: "manager" | "caretaker" }) => void;
  onRemove?: () => void;
}) {
  const [phone, setPhone] = useState(member?.phone ?? "+94 ");
  const [role, setRole] = useState<"manager" | "caretaker">(
    member?.role === "manager" ? "manager" : "caretaker",
  );
  const [ids, setIds] = useState<string[]>(member?.propertyIds ?? []);
  return (
    <InlineEditor
      title={
        !member
          ? "Invite someone"
          : member.kind === "invite"
            ? `Invite for ${member.phone}`
            : `Edit ${member.name || member.phone}`
      }
      saveLabel={!member ? "Send invite" : member.kind === "invite" ? "Send again" : "Save"}
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
      {member?.kind === "member" ? null : (
        <Text style={ui.faint}>
          We'll open WhatsApp with an invite link they can tap to join. Removing an invite stops its
          link from working.
        </Text>
      )}
    </InlineEditor>
  );
}
