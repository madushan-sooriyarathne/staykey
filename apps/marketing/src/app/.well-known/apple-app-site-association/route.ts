// Lets iOS open https://staykey.direct/join/<token> in the app instead of the browser. Apple
// fetches this file once the app is installed; it must be JSON with no redirect.

const BUNDLE_ID = "direct.staykey.app";

export const dynamic = "force-dynamic";

export function GET() {
  const team = process.env.STAYKEY_APPLE_TEAM_ID;
  if (!team) return new Response(null, { status: 404 });
  return Response.json({
    applinks: {
      details: [{ appIDs: [`${team}.${BUNDLE_ID}`], components: [{ "/": "/join/*" }] }],
    },
  });
}
