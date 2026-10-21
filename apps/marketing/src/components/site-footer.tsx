import Link from "next/link";
import { COMPANY, SUPPORT } from "@/lib/company";

const LINKS = [
  { href: "/pricing", label: "Pricing" },
  { href: "/support", label: "Support" },
  { href: "/terms", label: "Terms" },
  { href: "/privacy", label: "Privacy" },
  { href: "/delete-account", label: "Delete your account" },
];

export function SiteFooter() {
  return (
    <footer className="mx-auto max-w-6xl border-cloud border-t px-5 py-10 text-sm md:px-6">
      <div className="flex flex-col gap-6 md:flex-row md:items-start md:justify-between">
        <div className="flex max-w-sm flex-col gap-2">
          <span className="font-semibold text-obsidian">StayKey</span>
          <p className="text-fog">
            StayKey is a product of {COMPANY.name}, {COMPANY.registration}, {COMPANY.address}.
          </p>
          <a
            href={`mailto:${SUPPORT.email}`}
            className="focus-ring w-fit rounded-badge text-iron underline-offset-4 hover:underline"
          >
            {SUPPORT.email}
          </a>
        </div>
        <nav aria-label="Footer">
          <ul className="grid grid-cols-2 gap-x-8 gap-y-2 sm:flex sm:flex-wrap sm:gap-x-6">
            {LINKS.map((l) => (
              <li key={l.href}>
                <Link
                  href={l.href}
                  className="focus-ring rounded-badge text-iron underline-offset-4 hover:text-obsidian hover:underline"
                >
                  {l.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
      </div>
      <p className="mt-8 text-fog">
        © {new Date().getFullYear()} {COMPANY.name}
      </p>
    </footer>
  );
}
