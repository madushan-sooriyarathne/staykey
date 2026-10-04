/**
 * StayKey embed loader.
 *
 * Owners paste:
 *   <script src="https://cdn.staykey.direct/widget.js" async></script>
 *   <div data-staykey="kingfisher"></div>
 *
 * Optional attributes on the container:
 *   data-staykey-mode="widget" | "page"   Compact date picker or the full booking flow (default "widget")
 *   data-staykey-origin="http://kingfisher.localhost:3001"   Override the booking origin (local development)
 *
 * The iFrame posts { source: "staykey", type, ... } messages. "resize" sets the frame height so the
 * host page never shows a nested scrollbar; "event" is re-dispatched on the container as a
 * `staykey:event` CustomEvent and pushed to window.dataLayer when present, for GA4 and Meta Pixel.
 * Card payment never runs inside the frame: the booking page opens checkout in a top-level window.
 */

const ROOT_DOMAIN = "staykey.direct";
const MOUNTED = "staykeyMounted";

type WidgetMessage =
  | { source: "staykey"; type: "resize"; height: number }
  | { source: "staykey"; type: "event"; name: string; detail?: Record<string, unknown> };

declare global {
  interface Window {
    dataLayer?: unknown[];
    StayKey?: { mount: typeof mountAll };
  }
}

function originFor(container: HTMLElement, slug: string): string {
  return container.dataset.staykeyOrigin ?? `https://${slug}.${ROOT_DOMAIN}`;
}

function isWidgetMessage(data: unknown): data is WidgetMessage {
  return (
    typeof data === "object" && data !== null && (data as { source?: unknown }).source === "staykey"
  );
}

function mount(container: HTMLElement): void {
  const slug = container.dataset.staykey;
  if (!slug || container.dataset[MOUNTED]) return;
  container.dataset[MOUNTED] = "true";

  const origin = originFor(container, slug);
  const mode = container.dataset.staykeyMode === "page" ? "page" : "widget";
  const src = new URL(`/embed?mode=${mode}`, origin);
  src.searchParams.set("host", window.location.origin);

  const frame = document.createElement("iframe");
  frame.src = src.toString();
  frame.title = "Book your stay";
  frame.loading = "lazy";
  frame.allow = "payment";
  frame.style.cssText = "width:100%;border:0;display:block;height:220px;color-scheme:normal";
  container.appendChild(frame);

  window.addEventListener("message", (event: MessageEvent) => {
    if (event.origin !== origin || event.source !== frame.contentWindow) return;
    if (!isWidgetMessage(event.data)) return;

    if (event.data.type === "resize" && Number.isFinite(event.data.height)) {
      frame.style.height = `${Math.ceil(event.data.height)}px`;
      return;
    }

    if (event.data.type === "event") {
      const { name, detail } = event.data;
      container.dispatchEvent(
        new CustomEvent("staykey:event", { detail: { name, ...detail }, bubbles: true }),
      );
      window.dataLayer?.push({ event: `staykey_${name}`, ...detail });
    }
  });
}

function mountAll(root: ParentNode = document): void {
  for (const el of root.querySelectorAll<HTMLElement>("[data-staykey]")) mount(el);
}

window.StayKey = { mount: mountAll };

if (document.readyState === "loading") {
  document.addEventListener("DOMContentLoaded", () => mountAll());
} else {
  mountAll();
}

export {};
