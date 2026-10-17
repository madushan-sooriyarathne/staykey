import { isDay } from "./dates";

type SearchParams = { [key: string]: string | string[] | undefined };

function one(params: SearchParams, name: string): string | undefined {
  const v = params[name];
  return typeof v === "string" ? v : undefined;
}

function whole(params: SearchParams, name: string): number | undefined {
  const n = Number.parseInt(one(params, name) ?? "", 10);
  return Number.isFinite(n) && n >= 0 && n <= 30 ? n : undefined;
}

/** The stay a link carries, such as the widget's link to the booking page. Bad values drop. */
export function stayFromParams(params: SearchParams) {
  const checkIn = one(params, "checkIn");
  const checkOut = one(params, "checkOut");
  return {
    unitId: one(params, "unit") ?? one(params, "unitId"),
    checkIn: isDay(checkIn) ? checkIn : undefined,
    checkOut: isDay(checkOut) ? checkOut : undefined,
    adults: whole(params, "adults") ?? whole(params, "guests"),
    children: whole(params, "children"),
    from: one(params, "from") === "widget" ? ("widget" as const) : ("page" as const),
    hold: one(params, "hold"),
    expires: one(params, "expires"),
  };
}

export function param(params: SearchParams, name: string): string | undefined {
  return one(params, name);
}
