import { Body, Card, Chip, Heading, Screen } from "@/components/ui";

/** Placeholder for tabs that land in the next milestones. */
export function ComingNext({ title, summary }: { title: string; summary: string }) {
  return (
    <Screen title={title}>
      <Card>
        <Chip label="Next milestone" tone="ember" />
        <Heading>{title} is on the way</Heading>
        <Body>{summary}</Body>
      </Card>
    </Screen>
  );
}
