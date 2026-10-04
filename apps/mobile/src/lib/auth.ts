/**
 * Phone sign-in. The API doesn't have auth endpoints yet, so this is a local stand-in with the
 * same shape the real client will have: request a code, then verify it.
 *
 * TODO(auth): replace with calls to POST /v1/auth/otp and POST /v1/auth/otp/verify once they
 * exist in packages/api-spec, and store the returned session token in expo-secure-store.
 */

const wait = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));

export async function requestCode(_phone: string): Promise<void> {
  await wait(500);
}

/** In development any 6-digit code is accepted, except 000000, which simulates a wrong code. */
export async function verifyCode(_phone: string, code: string): Promise<boolean> {
  await wait(650);
  return /^\d{6}$/.test(code) && code !== "000000";
}
