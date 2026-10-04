import { createStayKeyClient, type Property } from "@staykey/api-client";
import { useCallback, useEffect, useState } from "react";

export const api = createStayKeyClient(process.env.EXPO_PUBLIC_API_URL ?? "http://localhost:8080");

type PropertiesState =
  | { status: "loading" }
  | { status: "error"; message: string }
  | { status: "ready"; items: Property[] };

/** Loads the owner's properties and exposes a reload function. */
export function useProperties() {
  const [state, setState] = useState<PropertiesState>({ status: "loading" });

  const reload = useCallback(async () => {
    try {
      const { data, response } = await api.GET("/v1/properties");
      if (data) setState({ status: "ready", items: data.items });
      else
        setState({
          status: "error",
          message: `Could not load properties (HTTP ${response.status}).`,
        });
    } catch {
      setState({
        status: "error",
        message:
          "Can't reach the StayKey API. Check EXPO_PUBLIC_API_URL and that the API is running.",
      });
    }
  }, []);

  useEffect(() => {
    reload();
  }, [reload]);

  return { state, reload };
}
