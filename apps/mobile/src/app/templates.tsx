import { router } from "expo-router";
import { useState } from "react";
import { Button } from "@/components/controls";
import { I } from "@/components/icons";
import { Appear, Card, Hint, List, ListRow, Page } from "@/components/kit";
import { useData } from "@/data/store";
import { TemplateText } from "@/features/templates/template-text";

const CHANNELS = { email: "Email", whatsapp: "WhatsApp" } as const;

/** Guest emails and WhatsApp messages with fields that fill in from each booking. */
export default function Templates() {
  const templates = useData((s) => s.templates);
  const [preview, setPreview] = useState(
    templates.find((t) => t.id === "prearrival")?.id ?? templates[0]?.id,
  );
  const current = templates.find((t) => t.id === preview);

  return (
    <Page title="Message templates">
      <Appear index={0}>
        <List>
          {templates.map((t) => (
            <ListRow
              key={t.id}
              testID={`template-${t.id}`}
              title={t.name}
              subtitle={`${t.channels.map((c) => CHANNELS[c]).join(" and ")}, ${t.timing.toLowerCase()}`}
              chevron={false}
              trailing={preview === t.id ? <Hint>Previewing</Hint> : undefined}
              onPress={() => setPreview(t.id)}
            />
          ))}
        </List>
      </Appear>
      {current ? (
        <Appear index={1}>
          <Card title={current.name} meta="Preview">
            <TemplateText body={current.body} />
            <Button
              compact
              variant="ghost"
              title="Edit template"
              icon={I.edit}
              testID="template-edit"
              onPress={() =>
                router.push({ pathname: "/template/[id]", params: { id: current.id } })
              }
            />
          </Card>
        </Appear>
      ) : null}
      <Hint>Highlighted fields fill in from each booking when the message is sent.</Hint>
    </Page>
  );
}
