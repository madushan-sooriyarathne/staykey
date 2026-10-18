"use server";

import { redirect } from "next/navigation";
import { confirmEmail } from "@/lib/api";

/**
 * Confirms the email behind the token, then leaves the token's address for a result page, so
 * it isn't kept in the address bar or the browser history.
 */
export async function confirm(form: FormData): Promise<void> {
  const token = String(form.get("token") ?? "");
  const done = await confirmEmail(token);
  if (done.ok) redirect("/verify-email?result=confirmed");
  redirect(done.status === 404 ? "/verify-email?result=expired" : "/verify-email?result=error");
}
