import type { ReactNode } from "react";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";
import { LEGAL_REVIEWED } from "@/lib/company";
import { formatDate } from "@/lib/format";

/** A legal text: its title, version and date, the draft notice until reviewed, and sections. */
export function LegalPage({
  title,
  version,
  updated,
  intro,
  children,
}: {
  title: string;
  version: string;
  updated: string;
  intro?: ReactNode;
  children: ReactNode;
}) {
  return (
    <>
      <SiteHeader />
      <main className="mx-auto max-w-3xl px-5 pt-10 pb-20 md:px-6 md:pt-16">
        <h1 className="font-semibold text-4xl text-obsidian leading-tight md:text-5xl">{title}</h1>
        <p className="mt-3 text-steel">
          Version {version}, last updated {formatDate(updated)}
        </p>
        {LEGAL_REVIEWED ? null : (
          <p className="mt-6 rounded-card border border-cloud bg-snow px-5 py-4 text-iron">
            Draft, under legal review. It describes how StayKey works today and may change before it
            is final.
          </p>
        )}
        {intro ? <div className="mt-8 text-body-lg text-steel">{intro}</div> : null}
        <div className="legal mt-10 flex flex-col gap-10">{children}</div>
      </main>
      <SiteFooter />
    </>
  );
}

/** One numbered section of a legal text, so people can refer to "section 4". */
export function Clause({ n, title, children }: { n: number; title: string; children: ReactNode }) {
  return (
    <section aria-labelledby={`s${n}`} className="flex flex-col gap-3">
      <h2 id={`s${n}`} className="font-semibold text-2xl text-obsidian">
        {n}. {title}
      </h2>
      <div className="flex flex-col gap-3 text-graphite">{children}</div>
    </section>
  );
}
