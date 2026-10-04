import type { NewProperty } from "@staykey/api-client";
import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { useSession } from "@/lib/session";
import { api } from "./client";
import { unwrap } from "./errors";

/** Keys include the account, so switching accounts never shows another account's cache. */
export const propertyKeys = {
  all: ["properties"] as const,
  list: (accountId: string) => ["properties", accountId] as const,
};

export const fetchProperties = async () => (await unwrap(api.GET("/v1/properties"))).items;

/**
 * Properties in the active account, from the API. Screens still read the on-device store until
 * phase 2 moves the Properties tab and settings across.
 */
export function useServerProperties() {
  const accountId = useSession((s) => s.accountId);
  return useQuery({
    queryKey: propertyKeys.list(accountId),
    queryFn: fetchProperties,
    enabled: Boolean(accountId),
  });
}

export const createProperty = (body: NewProperty) => unwrap(api.POST("/v1/properties", { body }));

export function useCreateProperty() {
  const client = useQueryClient();
  return useMutation({
    mutationFn: createProperty,
    onSuccess: () => client.invalidateQueries({ queryKey: propertyKeys.all }),
  });
}
