import { storeLinks } from "@/lib/stores";

const STORES = [
  { key: "ios", live: "Download on the App Store", soon: "Coming soon to the App Store" },
  { key: "android", live: "Get it on Google Play", soon: "Coming soon to Google Play" },
] as const;

/**
 * The App Store and Google Play buttons. A store without a listing yet shows a quiet "coming
 * soon" label instead of a link. Swap in the stores' official badge artwork once both listings
 * are live.
 */
export function StoreButtons({ tone = "dark" }: { tone?: "dark" | "light" }) {
  const links = storeLinks();
  return (
    <div className="flex flex-wrap gap-3">
      {STORES.map((s) => {
        const href = links[s.key];
        if (!href) {
          return (
            <span
              key={s.key}
              className={`rounded-button border px-4 py-3 text-sm ${
                tone === "dark" ? "border-mist text-iron" : "border-steel text-ash"
              }`}
            >
              {s.soon}
            </span>
          );
        }
        return (
          <a
            key={s.key}
            href={href}
            className={`focus-ring rounded-button px-4 py-3 font-medium text-sm ${
              tone === "dark" ? "bg-obsidian text-snow shadow-button" : "bg-snow text-obsidian"
            }`}
          >
            {s.live}
          </a>
        );
      })}
    </div>
  );
}
