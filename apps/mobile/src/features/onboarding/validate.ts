/** Empty, or something that looks like an address; the API has the final say. */
export function isOptionalEmail(email: string): boolean {
  const v = email.trim();
  return v === "" || /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v);
}
