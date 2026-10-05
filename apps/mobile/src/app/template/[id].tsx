import { router, useLocalSearchParams } from "expo-router";
import { useState } from "react";
import { Text, View } from "react-native";
import { Field, Pill } from "@/components/controls";
import { Card, Hint, Page, SectionHeader, ui } from "@/components/kit";
import { TEMPLATE_VARIABLES } from "@/data/defaults";
import { useBookings, useProperties } from "@/data/hooks";
import { useData } from "@/data/store";
import { fillTemplate } from "@/lib/contact";
import { haptics } from "@/lib/haptics";

/** Edits one message template, with fields to insert and a preview filled from a real booking. */
export default function TemplateEditor() {
  const { id } = useLocalSearchParams<{ id: string }>();
  const template = useData((s) => s.templates.find((t) => t.id === id));
  const property = useProperties()[0];
  const booking = useBookings().find((b) => b.status === "confirmed" && b.guest.phone);
  const updateTemplate = useData((s) => s.updateTemplate);
  const [body, setBody] = useState(template?.body ?? "");
  if (!template || !property) return null;

  return (
    <Page
      title={template.name}
      action={{
        label: "Save",
        disabled: body === template.body || body.trim().length < 5,
        testID: "template-save",
        onPress: () => {
          haptics.success();
          updateTemplate(template.id, body);
          router.back();
        },
      }}
    >
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
            onPress={() => setBody((b) => `${b.trimEnd()} {${v.key}}`)}
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
