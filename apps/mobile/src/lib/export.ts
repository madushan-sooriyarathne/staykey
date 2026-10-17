import { File, Paths } from "expo-file-system";
import * as Sharing from "expo-sharing";
import { Platform } from "react-native";
import { exportAccount } from "@/api/accounts";

/**
 * Saves the active account's data as a JSON file and opens the share sheet, so the owner can keep
 * it in Files, mail it to themselves or send it on.
 */
export async function shareAccountExport(): Promise<{ ok: true } | { ok: false; message: string }> {
  if (Platform.OS === "web" || !(await Sharing.isAvailableAsync())) {
    return { ok: false, message: "Download your data from the StayKey app on your phone." };
  }
  const data = await exportAccount();
  const day = data.exportedAt.slice(0, 10);
  const file = new File(Paths.cache, `staykey-data-${day}.json`);
  if (file.exists) file.delete();
  file.create();
  file.write(JSON.stringify(data, null, 2));
  await Sharing.shareAsync(file.uri, {
    mimeType: "application/json",
    UTI: "public.json",
    dialogTitle: "Your StayKey data",
  });
  return { ok: true };
}
