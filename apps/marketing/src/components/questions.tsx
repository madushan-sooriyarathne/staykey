import type { ReactNode } from "react";

export type Question = { q: string; a: ReactNode };

/** Questions and answers as native disclosure widgets: keyboard ready, no JavaScript. */
export function Questions({ items }: { items: readonly Question[] }) {
  return (
    <div className="divide-y divide-cloud rounded-card-lg border border-cloud bg-snow">
      {items.map((item) => (
        <details key={item.q} className="group px-6 py-1 md:px-8">
          <summary className="focus-ring flex cursor-pointer list-none items-center justify-between gap-4 rounded-badge py-4 font-medium text-obsidian [&::-webkit-details-marker]:hidden">
            {item.q}
            <span aria-hidden className="text-fog transition-transform group-open:rotate-45">
              +
            </span>
          </summary>
          <div className="pb-5 text-steel">{item.a}</div>
        </details>
      ))}
    </div>
  );
}
