import AsyncStorage from "@react-native-async-storage/async-storage";
import { createJSONStorage } from "zustand/middleware";

/** Persistent storage for zustand stores. Survives app restarts on device and web. */
export const persistentStorage = createJSONStorage(() => AsyncStorage);
