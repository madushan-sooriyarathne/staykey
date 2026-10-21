/**
 * Where guests type their details and see their stay: who handles their data, and the policy.
 * StayKey processes guests' details for the owner, who decides what happens to them.
 */
export function PrivacyNote({ propertyName }: { propertyName: string }) {
  return (
    <p className="text-center text-sm text-steel">
      StayKey handles your details for {propertyName}.{" "}
      <a
        href="https://staykey.direct/privacy"
        className="underline underline-offset-4 hover:text-graphite"
      >
        Privacy
      </a>
    </p>
  );
}
