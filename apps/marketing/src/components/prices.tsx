import type { BillingPeriod, Plan } from "@staykey/api-client";
import Link from "next/link";
import { formatRupees } from "@/lib/format";
import { INCLUDED, priceFor, unitsLabel, yearlySaving } from "@/lib/plans";

/**
 * The plans side by side in one panel, with the period switch as links so the page needs no
 * JavaScript. With no plans (the API can't be reached), it points to the app instead.
 */
export function Prices({
  plans,
  period,
  switchHref,
}: {
  plans: Plan[] | null;
  period: BillingPeriod;
  /** Where the period links go; omit to hide the switch. */
  switchHref?: (period: BillingPeriod) => string;
}) {
  if (!plans) {
    return (
      <div className="rounded-card-lg border border-cloud bg-snow p-8">
        <p className="font-semibold text-obsidian text-xl">See prices in the app</p>
        <p className="mt-2 text-steel">
          We can't show prices right now. The app shows every plan, and every plan includes
          everything StayKey does.
        </p>
      </div>
    );
  }
  return (
    <div className="flex flex-col gap-5">
      {switchHref ? (
        <nav
          aria-label="Billing period"
          className="flex w-fit gap-1 rounded-button border border-cloud bg-snow p-1"
        >
          {(["monthly", "yearly"] as const).map((p) => (
            <Link
              key={p}
              href={switchHref(p)}
              scroll={false}
              aria-current={p === period ? "page" : undefined}
              className={`focus-ring rounded-[10px] px-4 py-2 text-sm ${
                p === period ? "bg-obsidian text-snow" : "text-iron hover:text-obsidian"
              }`}
            >
              {p === "monthly" ? "Monthly" : "Yearly"}
            </Link>
          ))}
        </nav>
      ) : null}
      <div className="grid overflow-hidden rounded-card-lg border border-cloud bg-snow md:grid-cols-3">
        {plans.map((plan, i) => {
          const saving = period === "yearly" ? yearlySaving(plan) : "";
          return (
            <section
              key={plan.id}
              aria-labelledby={`plan-${plan.id}`}
              className={`flex flex-col gap-3 p-7 md:p-8 ${i > 0 ? "border-cloud border-t md:border-t-0 md:border-l" : ""}`}
            >
              <h3 id={`plan-${plan.id}`} className="font-semibold text-obsidian text-xl">
                {plan.name}
              </h3>
              <p className="text-steel">{unitsLabel(plan)}</p>
              <p className="mt-2 flex items-baseline gap-2">
                <span className="font-semibold text-4xl text-obsidian">
                  {formatRupees(priceFor(plan, period), plan.currency)}
                </span>
                <span className="text-steel">{period === "yearly" ? "a year" : "a month"}</span>
              </p>
              {saving ? (
                <span className="w-fit rounded-badge bg-ember-tint px-2 py-1 text-caption text-ember-ink">
                  {saving}
                </span>
              ) : null}
            </section>
          );
        })}
      </div>
      <div className="flex flex-col gap-3 rounded-card-lg border border-cloud p-7 md:flex-row md:items-start md:justify-between md:p-8">
        <p className="font-semibold text-obsidian">Every plan includes</p>
        <ul className="grid gap-x-8 gap-y-2 text-steel sm:grid-cols-2 md:max-w-2xl">
          {INCLUDED.map((item) => (
            <li key={item} className="flex gap-2">
              <span aria-hidden className="text-ember">
                ✓
              </span>
              {item}
            </li>
          ))}
        </ul>
      </div>
    </div>
  );
}
