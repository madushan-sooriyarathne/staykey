import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";
import { messageFor } from "@/api/errors";
import { useTemplates, useUpdateTemplate } from "@/api/messages";
import { Field, InfoNote, Pill } from "@/components/controls";
import { I } from "@/components/icons";
import { Card, Hint, Page, SectionHeader, ui } from "@/components/kit";
import { TEMPLATE_VARIABLES } from "@/data/defaults";
import { useBookings, useProperties } from "@/data/hooks";
import { fillTemplate } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

/** Edits one message template, with fields to insert and a preview filled from a real booking. */
export default function TemplateEditor() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const template = useTemplates().find((t) => t.id === id);
  const property = useProperties()[0];
  const booking = useBookings().find((b) => b.status === "confirmed" && b.guest.phone);
  const update = useUpdateTemplate();
  const [edited, setBody] = useState<string>();
  const [error, setError] = useState<string | null>(null);
  if (!template || !property) return null;
  const body = edited ?? template.body;

  return (
    <Page
      title={template.name}
      action={{
        label: "Save",
        disabled: body === template.body || body.trim().length < 5 || update.isPending,
        testID: "template-save",
        onPress: () => {
          setError(null);
          update.mutate(
            { id: template.id, body },
            {
              onSuccess: () => {
                haptics.success();
                router.back();
              },
              onError: (e) => {
                haptics.error();
                setError(messageFor(e));
              },
            },
          );
        },
      }}
    >
      {error ? (
        <InfoNote icon={I.warning} testID="template-error">
          {error}
        </InfoNote>
      ) : null}
      <Field
        testID="template-body"
        label="Message"
        value={body}
        onChangeText={setBody}
        multiline
        style={{ minHeight: 140, textAlignVertical: "top" }}
      />
      <SectionHeader title="Insert a field" />
      <View style={{ flexDirection: "row", flexWrap: "wrap", gap: 8 }}>
        {TEMPLATE_VARIABLES.map((v) => (
          <Pill
            key={v.key}
            label={v.label}
            selected={body.includes(`{${v.key}}`)}
            onPress={() => setBody(`${body.trimEnd()} {${v.key}}`)}
          />
        ))}
      </View>
      <Card title="Preview" meta={booking ? booking.guest.name : "Sample guest"}>
        <Text style={ui.body}>{fillTemplate(body, booking, property)}</Text>
      </Card>
      <Hint>Sent {template.timing.toLowerCase()}.</Hint>
    </Page>
  );
}
