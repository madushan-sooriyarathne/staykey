import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Email confirmation",
  robots: { index: false, follow: false },
};

const results = {
  confirmed: {
    title: "Your email is confirmed",
    body: "Booking updates and account notices from StayKey will come to this address. You can close this page.",
  },
  expired: {
    title: "This link has expired",
    body: "This link has expired or was already used. Send a new one from Profile in the app.",
  },
  error: {
    title: "We couldn't confirm your email",
    body: "Something went wrong on our side. Please open the link from your email again in a few minutes.",
  },
} as const;

type ResultKey = keyof typeof results;

const isResult = (v: unknown): v is ResultKey => typeof v === "string" && v in results;

/** Where the confirmation form lands, without the token in the address. */
export default async function VerifyEmailResultPage(props: PageProps<"/verify-email">) {
  const { result } = await props.searchParams;
  const shown = results[isResult(result) ? result : "expired"];

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center gap-6 px-6 py-16">
      <span className="font-semibold text-lg text-obsidian">StayKey</span>
      <h1 className="font-semibold text-4xl text-obsidian leading-tight">{shown.title}</h1>
      <p className="text-steel">{shown.body}</p>
    </main>
  );
}
