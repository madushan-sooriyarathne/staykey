import type { Metadata } from "next";
import { confirm } from "../actions";

export const metadata: Metadata = {
  title: "Confirm your email",
  description: "Confirm the email you added to StayKey.",
  robots: { index: false, follow: false },
  referrer: "no-referrer",
};

/**
 * Where confirmation links land. Confirming takes a tap rather than happening on load, because
 * mail scanners open links before people do and the link works only once.
 */
export default async function VerifyEmailPage(props: PageProps<"/verify-email/[token]">) {
  const { token } = await props.params;

  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center gap-6 px-6 py-16">
      <span className="font-semibold text-lg text-obsidian">StayKey</span>
      <h1 className="font-semibold text-4xl text-obsidian leading-tight">Confirm your email</h1>
      <p className="text-steel">
        Confirm this is your email, so booking updates and account notices from StayKey reach you.
      </p>
      <form action={confirm}>
        <input type="hidden" name="token" value={token} />
        <button
          type="submit"
          className="w-full rounded-button bg-obsidian px-5 py-4 text-center text-snow shadow-button"
        >
          Confirm my email
        </button>
      </form>
    </main>
  );
}
