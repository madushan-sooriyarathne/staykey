import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Join your team on StayKey",
  description: "Open the StayKey app to accept your invite.",
  robots: { index: false, follow: false },
};

/**
 * Where WhatsApp invite links land. The page hands the token to the app, which shows who sent
 * the invite and signs the person in. Tokens are only ever in the link, never sent to a server
 * from this page.
 */
export default async function JoinPage(props: PageProps<"/join/[token]">) {
  const { token } = await props.params;
  const appLink = `staykey://join?token=${encodeURIComponent(token)}`;

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center gap-6 px-6 py-16">
      <span className="font-semibold text-lg text-obsidian">StayKey</span>
      <h1 className="font-semibold text-4xl text-obsidian leading-tight">
        You're invited to help run a property
      </h1>
      <p className="text-steel">
        Open the invite in the StayKey app, then confirm your phone number with the code we text
        you.
      </p>
      <a
        href={appLink}
        className="rounded-button bg-obsidian px-5 py-4 text-center text-snow shadow-button"
      >
        Open StayKey to join
      </a>
      <p className="text-sm text-steel">
        Don't have the app yet? Install StayKey from the App Store or Google Play, then tap this
        link again.
      </p>
    </main>
  );
}
