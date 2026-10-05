import type { NewProperty, Property, PropertyPatch } from "@staykey/api-client";
import { type QueryClient, useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { toPropertyConfig } from "@/data/from-api";
import type { PropertyConfig } from "@/data/types";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

/** Keys include the account, so switching accounts never shows another account's cache. */
export const propertyKeys = {
  all: ["properties"] as const,
  list: (accountId: string) => ["properties", accountId] as const,
};

export const fetchProperties = async () => (await unwrap(api.GET("/v1/properties"))).items;

const EMPTY: PropertyConfig[] = [];
// Module-level so TanStack Query keeps the mapped array stable between renders.
const toConfigs = (items: Property[]) => items.map(toPropertyConfig);

/** The active account's properties with their full setup, as the screens' model. */
export function useProperties(): PropertyConfig[] {
  const accountId = useSession((s) => s.accountId);
  const { data } = useQuery({
    queryKey: propertyKeys.list(accountId),
    queryFn: fetchProperties,
    enabled: Boolean(accountId),
    select: toConfigs,
  });
  return data ?? EMPTY;
}

/** Whether the property list has loaded (from the server or the saved cache). */
export function usePropertiesReady(): boolean {
  const accountId = useSession((s) => s.accountId);
  const { isSuccess } = useQuery({
    queryKey: propertyKeys.list(accountId),
    queryFn: fetchProperties,
    enabled: Boolean(accountId),
  });
  return isSuccess;
}

/** Puts a created or updated property into the active account's cached list. */
export function storeProperty(client: QueryClient, accountId: string, property: Property) {
  client.setQueryData<Property[]>(propertyKeys.list(accountId), (items = []) =>
    items.some((p) => p.id === property.id)
      ? items.map((p) => (p.id === property.id ? property : p))
      : [property, ...items],
  );
}

export const createProperty = (body: NewProperty) => unwrap(api.POST("/v1/properties", { body }));

export function useCreateProperty() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: createProperty,
    onSuccess: (property) => storeProperty(client, accountId, property),
  });
}

export const updateProperty = (id: string, patch: PropertyPatch) =>
  unwrap(api.PATCH("/v1/properties/{id}", { params: { path: { id } }, body: patch }));

/** Saves some of a property's settings; the cache takes the server's answer. */
export function useUpdateProperty() {
  const client = useQueryClient();
  const accountId = useSession((s) => s.accountId);
  return useMutation({
    mutationFn: ({ id, patch }: { id: string; patch: PropertyPatch }) => updateProperty(id, patch),
    onSuccess: (property) => storeProperty(client, accountId, property),
  });
}
