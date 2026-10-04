/**
 * Prototype mode seeds sample bookings and shows restart and preview tools in More. On in
 * development, and in builds made for prototype testing with EXPO_PUBLIC_PROTOTYPE_TOOLS=true.
 */
export const PROTOTYPE = __DEV__ || process.env.EXPO_PUBLIC_PROTOTYPE_TOOLS === "true";
