// The uptime monitor calls this. The site's pages work without the API, so this checks only
// that the site itself answers.
export const dynamic = "force-dynamic";

export function GET() {
  return Response.json({ status: "ok" }, { headers: { "Cache-Control": "no-store" } });
}
