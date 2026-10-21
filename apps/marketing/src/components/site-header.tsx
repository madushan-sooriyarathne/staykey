import Link from "next/link";

// Pricing stays visible on phones; the rest wait for room.
const NAV = [
  { href: "/#features", label: "Features", always: false },
  { href: "/pricing", label: "Pricing", always: true },
  { href: "/support", label: "Support", always: false },
];

export function SiteHeader() {
  return (
    <header className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-5 py-5 md:px-6">
      <Link href="/" className="focus-ring rounded-badge font-semibold text-lg text-obsidian">
        StayKey
      </Link>
      <nav aria-label="Main" className="flex items-center gap-1 md:gap-2">
        {NAV.map((n) => (
          <Link
            key={n.href}
            href={n.href}
            className={`focus-ring rounded-button px-3 py-2 text-iron text-sm hover:text-obsidian ${
              n.always ? "inline-block" : "hidden sm:inline-block"
            }`}
          >
            {n.label}
          </Link>
        ))}
        <Link
          href="/#download"
          className="focus-ring rounded-button bg-obsidian px-4 py-3 text-sm text-snow shadow-button"
        >
          Get the app
        </Link>
      </nav>
    </header>
  );
}
