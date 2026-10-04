import { create } from "zustand";
import { useSession } from "@/lib/session";
import { useData } from "./store";

/** Which property the Today, Bookings and Calendar tabs are showing. Not persisted. */
export const useFilter = create<{
  propertyId: string | "all";
  set: (id: string | "all") => void;
}>()((set) => ({
  propertyId: "all",
  set: (propertyId) => set({ propertyId }),
}));

export const useProperties = () => useData((s) => s.properties);
export const useProperty = (id?: string) => useData((s) => s.properties.find((p) => p.id === id));
export const useBooking = (id?: string) => useData((s) => s.bookings.find((b) => b.id === id));

/** The property for single-property screens: the filtered one, else the first. */
export function useFocusedProperty() {
  const properties = useProperties();
  const filter = useFilter((s) => s.propertyId);
  return properties.find((p) => p.id === filter) ?? properties[0];
}

export type Permission = "prices" | "settings" | "billing" | "team" | "manage";

/** Managers run the property but not billing; caretakers see stays without prices. */
export function useCan() {
  const role = useSession((s) => s.role);
  return (what: Permission) => {
    if (role === "owner") return true;
    if (role === "manager") return what !== "billing" && what !== "team";
    return false;
  };
}
