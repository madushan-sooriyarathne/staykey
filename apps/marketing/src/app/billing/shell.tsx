import type { ReactNode } from "react";

/** The narrow column every billing step sits in. */
export function Shell({ title, children }: { title: string; children: ReactNode }) {
  return (
    <main className="mx-auto flex min-h-dvh max-w-md flex-col justify-center gap-6 px-6 py-16">
      <span className="font-semibold text-lg text-obsidian">StayKey</span>
      <h1 className="font-semibold text-4xl text-obsidian leading-tight">{title}</h1>
      {children}
    </main>
  );
}
