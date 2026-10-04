import { bookingPageUrl, ROOT_DOMAIN } from "@/lib/tenant";

/** The root of the booking domain. Real traffic arrives on property subdomains. */
export default function Home() {
  const example = bookingPageUrl("kingfisher");
  return (
    <main className="mx-auto flex min-h-screen max-w-xl flex-col justify-center gap-4 px-6">
      <h1 className="font-semibold text-4xl text-obsidian">StayKey booking pages</h1>
      <p className="text-steel">
        Each property is served on its own subdomain of {ROOT_DOMAIN}. Create a property through the
        API, then open its page, for example{" "}
        <a className="text-obsidian underline underline-offset-4" href={example}>
          {example}
        </a>
        .
      </p>
    </main>
  );
}
