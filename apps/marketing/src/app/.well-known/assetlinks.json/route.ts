// Lets Android open https://staykey.direct/join/<token> in the app. Google checks the app's
// signing certificate against these fingerprints.

const PACKAGE = "direct.staykey.app";

export const dynamic = "force-dynamic";

export function GET() {
  const fingerprints = (process.env.STAYKEY_ANDROID_CERT_SHA256 ?? "")
    .split(",")
    .map((f) => f.trim())
    .filter(Boolean);
  if (fingerprints.length === 0) return new Response(null, { status: 404 });
  return Response.json([
    {
      relation: ["delegate_permission/common.handle_all_urls"],
      target: {
        namespace: "android_app",
        package_name: PACKAGE,
        sha256_cert_fingerprints: fingerprints,
      },
    },
  ]);
}
