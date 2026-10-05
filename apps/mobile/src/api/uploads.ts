import { api } from "./client";
import { ApiError, OfflineError, unwrap } from "./errors";

type Kind = "photo" | "logo";
type ContentType = "image/jpeg" | "image/png" | "image/webp" | "image/heic";

const TYPES: Record<string, ContentType> = {
  "image/jpeg": "image/jpeg",
  "image/jpg": "image/jpeg",
  "image/png": "image/png",
  "image/webp": "image/webp",
  "image/heic": "image/heic",
};

function contentType(blobType: string, uri: string): ContentType {
  const known = TYPES[blobType.toLowerCase()];
  if (known) return known;
  const ext = uri.split("?")[0]?.split(".").pop()?.toLowerCase();
  if (ext === "png") return "image/png";
  if (ext === "webp") return "image/webp";
  if (ext === "heic") return "image/heic";
  return "image/jpeg";
}

/**
 * Uploads a picked image straight to storage: ask the API for a link that accepts exactly this
 * file, send it there, and return the key to save on the property.
 */
export async function uploadImage(uri: string, kind: Kind): Promise<{ key: string; url: string }> {
  let blob: Blob;
  try {
    blob = await (await fetch(uri)).blob();
  } catch {
    throw new ApiError(0, { code: "file_unreadable", message: "We couldn't read that photo." });
  }
  const type = contentType(blob.type, uri);
  const ticket = await unwrap(
    api.POST("/v1/uploads", { body: { kind, contentType: type, size: blob.size } }),
  );
  let res: Response;
  try {
    res = await fetch(ticket.uploadUrl, {
      method: ticket.method,
      headers: ticket.headers,
      body: blob,
    });
  } catch {
    throw new OfflineError();
  }
  if (!res.ok) {
    throw new ApiError(res.status, {
      code: "upload_failed",
      message: "The upload didn't finish. Try again.",
    });
  }
  return { key: ticket.key, url: ticket.url };
}
