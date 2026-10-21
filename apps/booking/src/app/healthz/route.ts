// The uptime monitor calls this. The booking page can't take bookings without the API, so a
// slow or missing API makes this page unhealthy too.
export const dynamic = "force-dynamic";

const API_URL = process.env.STAYKEY_API_URL ?? "http://localhost:8080";

export async function GET() {
  const isApiUp = await fetch(`${API_URL}/healthz`, {
    cache: "no-store",
    signal: AbortSignal.timeout(2000),
  }).then(
    (response) => response.ok,
    () => false,
  );
  return Response.json(
    isApiUp ? { status: "ok", api: "up" } : { status: "degraded", api: "down" },
    { status: isApiUp ? 200 : 503, headers: { "Cache-Control": "no-store" } },
  );
}
