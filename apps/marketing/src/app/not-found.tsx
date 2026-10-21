import Link from "next/link";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";

export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto flex max-w-3xl flex-col gap-5 px-5 py-24 md:px-6">
        <h1 className="font-semibold text-4xl text-obsidian md:text-5xl">This page isn't here</h1>
        <p className="text-body-lg text-steel">
          The link may be old or mistyped. Looking for a property's booking page? Its address ends
          in .staykey.direct.
        </p>
        <div className="flex flex-wrap gap-3">
          <Link
            href="/"
            className="focus-ring rounded-button bg-obsidian px-4 py-3 text-sm text-snow shadow-button"
          >
            Go to StayKey
          </Link>
          <Link
            href="/support"
            className="focus-ring rounded-button border border-mist px-4 py-3 text-iron text-sm"
          >
            Get help
          </Link>
        </div>
      </main>
      <SiteFooter />
    </>
  );
}
