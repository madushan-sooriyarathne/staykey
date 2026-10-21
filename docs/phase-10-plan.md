# Build phase 10: the marketing site

Drafted 21 Oct 2026 on `feat/phase-9-card-payments` (PR #10). Written for the agent that executes it; the checklist is `docs/ROADMAP.md`, the state of the code `docs/HANDOVER.md`.

## Goal

An owner who hears about StayKey can land on staykey.direct, understand what it does, see the price, install the app, and reach support. App Store and Google Play review can find a privacy policy, terms, a support page and an account deletion page. Every shared link gets a clean preview.

The pages:

- `/`: what StayKey does, how it works, features, prices, questions and answers, and the download links.
- `/pricing`: plans from `GET /v1/public/plans`.
- `/support`, `/privacy`, `/terms` and `/delete-account`.
- `/terms/payouts`: the payout terms the app already asks owners to accept (version `2026-10`).
- `/download`: one link to share that sends each phone to its store.

The existing billing, join and email confirmation pages stay as they are, unindexed.

## Assumptions

Each one is a call made without asking. Record them in `docs/HANDOVER.md` under "Calls made without asking".

- **Prices come from the API.** `/` and `/pricing` show the LKR web prices from `GET /v1/public/plans` and say the App Store and Google Play show their own price in the visitor's currency. The page renders at request time with a one hour cache, so a build never needs the API. If the API can't be reached, the prices block says "See prices in the app" and the rest of the page renders.
- **No client JavaScript is needed.** The monthly or yearly choice on `/pricing` is a link (`?period=yearly`), rendered on the server.
- **Store links come from settings:** `STAYKEY_APP_STORE_URL`, `STAYKEY_PLAY_STORE_URL` and `STAYKEY_APP_STORE_ID`, the last for Apple's Smart App Banner. While a link is unset, its button reads "Coming soon to the App Store" and isn't a link. The buttons are plain text buttons in the brand style; swap in Apple's and Google's official badge artwork once the listings are live (both stores require their own badges when badges are used).
- **`/download` picks a store by user agent:** iPhone and iPad go to the App Store, Android to Google Play, everything else to `/#download`. It never sets a cookie.
- **Support is email first:** support@staykey.direct (already used in emails and the app). WhatsApp shows only when `STAYKEY_SUPPORT_WHATSAPP` is set. There is no contact form, so there is nothing to spam and nothing to store.
- **Legal pages are drafts until a lawyer reviews them.** They are written to fit Sri Lanka's Personal Data Protection Act No. 9 of 2022. Each shows its version and date. While `LEGAL_REVIEWED` in `src/lib/company.ts` is false, each also shows "Draft, under legal review" and isn't indexed. The company's legal name, address and registration number live in the same file.
- **Who controls which data:** StayKey is the controller of owners' and team members' data, and processes guests' data for the owner, who controls it. The privacy policy covers both, and the booking page links to it where guests type their details.
- **The payout terms page** carries the version the app records (`2026-10`). It says StayKey payouts aren't available yet, isn't in the sitemap or the navigation, and isn't indexed. Only the app's payout details screen links to it, and that screen only shows when payouts are offered.
- **Features describe what is built.** Card payments are described as the owner's own Payments.lk account; StayKey payouts aren't mentioned anywhere public while `STAYKEY_PAYOUTS_ENABLED` is off in production.
- **No analytics and no cookies** on public pages, so there is no consent banner. The billing page's sign-in cookies stay as they are.
- **English only.** Sinhala and Tamil can come later.
- **Screenshots are real.** Phone screens are captured from the app's web build against the seeded API, saved as WebP in `apps/marketing/public/screens/`, and shown in a plain CSS phone frame (no device artwork). How to capture them again goes in the handover.
- **The social image is generated** with `opengraph-image.tsx` (`next/og`), using DM Sans from a committed font file (SIL Open Font License, with its licence beside it) rather than a network fetch at build.

## Context

- The site today: `src/app/page.tsx` is a one-screen landing page with three benefit cards and no links out; `layout.tsx` sets DM Sans and basic metadata on `https://staykey.direct`. There is no `public/` folder, sitemap, robots file, icons or social image.
- Other routes: `billing/*` (owner sign-in and Payments.lk checkout, httpOnly cookies scoped to `/billing`, `src/proxy.ts` refreshes them), `join/[token]`, `verify-email/*`, `.well-known/*`. All are `robots: { index: false }` already.
- `src/lib/api.ts` is `server-only` and calls the API through `@staykey/api-client` with `forwardVisitor`; `src/lib/format.ts` has `formatRupees` and Colombo dates.
- Plans: `Plan` is `{id, name, units, currency, monthly, yearly}` in minor units. Web prices are Rs 2,900, 5,900 and 11,900 a month (`domain.Plans`); the app's store prices are $9, $19 and $39 (`apps/mobile/src/lib/purchases.ts`, with `INCLUDED`).
- Brand: tokens in `@staykey/tokens/theme.css`, rules in `docs/HANDOVER.md` section 2, the full reference in `docs/design/DESIGN.md` (zinc neutrals, 36px cards, 14px buttons, hairline borders, one ember accent, DM Sans display at 56 to 64px, weight 600).
- The app: `help.tsx` still tells owners to apply to PayHere for cards and uses a placeholder WhatsApp number (`+94 77 000 0000`); the paywall has no links to terms or privacy; nothing links to the site.
- Next 16 docs live in `apps/marketing/node_modules/next/dist/docs/`. The relevant ones are `01-app/01-getting-started/14-metadata-and-og-images.md`, `03-api-reference/03-file-conventions/01-metadata/` (sitemap, robots, opengraph-image, app icons), `02-guides/json-ld.md`, and caching without Cache Components (the site doesn't enable `cacheComponents`).
- Turbo passes `STAYKEY_*` to `build` already; `dev` lists variables one by one.

## Constraints

- Work in this order, one commit per step: shared layout and content, pages, metadata, booking app link, mobile links, docs.
- Read `apps/marketing/AGENTS.md` and the Next 16 docs above before writing framework code. No new runtime dependencies; `next/og`, `next/image` and the metadata conventions cover it.
- Tokens only from `@staykey/tokens`; no raw colours or sizes in components. Primary buttons stay obsidian; ember marks only what the owner should notice; Magenta Spark only for the one decorative accent.
- No em dashes anywhere, in copy or code. Plain, understated copy that says what the product does.
- Every page works at 360px wide, keyboard first, with visible focus, alt text on screenshots, and contrast of at least 4.5:1.
- Server components only, unless a step proves it can't be done without a client component.
- New settings go in `apps/marketing/.env.example` and turbo's `dev.passThroughEnv`.

## Steps

### 1. Shared layout and content
- `src/components/site-header.tsx` and `site-footer.tsx`:
  - the header has the wordmark, Features, Pricing, Support and "Get the app" (to `#download`);
  - the footer has Terms, Privacy, Support, Delete your account, the company line from `company.ts`, and the year.
  - The billing, join and email pages keep their own minimal shells.
- `src/lib/company.ts`: legal name, address, registration number, support email, `LEGAL_REVIEWED`, and the legal versions and dates (`PRIVACY_VERSION`, `TERMS_VERSION`, `PAYOUT_TERMS_VERSION = "2026-10"`).
- `src/lib/stores.ts`: reads the store settings on the server and returns each link or `null`. Also `storeFor(userAgent)`, with unit tests for iPhone, iPad, Android and desktop agents.
- Tests use `bun:test` in `src/lib/__tests__/`, excluded from `tsc` the way `apps/mobile` excludes its own; add a `test` script (`bun test src`).
- `src/lib/api.ts` gains `plans()`, which returns `Result<Plan[]>` with a one hour revalidation; check the Next 16 caching guide for the right option with openapi-fetch. `src/lib/format.ts` gains a yearly saving helper ("2 months free"), with tests.
- `src/components/store-buttons.tsx`: the two buttons, or their "coming soon" state.
- `src/components/phone-frame.tsx`: a CSS frame around a `next/image` screenshot.

### 2. Pages
- **`/`, top to bottom:**
  - the hero, "Take direct bookings in 10 minutes", with the store buttons and a Today screenshot;
  - how it works in three steps: install and sign in with your phone, add your property and photos, share your booking link;
  - features: booking page and widget, one calendar with Airbnb and Booking.com sync, bank transfer, pay at the property and card payments through your own Payments.lk account, requests or instant booking, team roles for managers and caretakers, WhatsApp and email guest messages, Activity and alerts;
  - "What StayKey replaces" (kept);
  - the prices block (the same component as `/pricing`, monthly);
  - questions and answers: commission, free period, OTA calendars, card payments, data ownership, cancelling;
  - the download section (`id="download"`).
- **`/pricing`:** monthly and yearly links; three plan cards (name, units, price, yearly saving); every plan's included features (from the app's `INCLUDED`, kept in step by hand); the free period ("Free until your first direct booking, for up to 60 days"); no commission on bookings; how to pay (in the app through the App Store or Google Play, or on the web in rupees at `/billing`); and questions about changing plans.
- **`/support`:**
  - email and, when set, WhatsApp;
  - the help topics from the app, brought up to date (Airbnb calendar, the widget, a WhatsApp booking, card payments with Payments.lk, inviting a caretaker, deleting your account);
  - a line on response times ("We reply within one working day, Sri Lanka time").
- **`/privacy`:**
  - who we are;
  - what we collect from owners, team members and guests, and why (the lawful basis for each);
  - processors: AWS for email, Cloudflare R2 for files, the host, Payments.lk and Payable for cards, RevenueCat, Apple and Google for purchases, Expo for push, oZone for SMS, and Meta once WhatsApp is on;
  - transfers outside Sri Lanka;
  - retention: 30 days after deleting an account, guest erasure by owners, and backups;
  - rights under the PDPA and how to use them; children; changes; contact.
- **`/terms`:**
  - the service, accounts and roles;
  - owners' responsibilities (accurate listings, their own agreements with guests, tax);
  - plans, the free period and grace, store and web purchases;
  - payments: StayKey isn't a party to bookings, and card payments go to the owner's own Payments.lk account;
  - acceptable use, content and licence, availability, liability, suspension and deletion;
  - governing law (Sri Lanka) and changes.
- **`/terms/payouts`:**
  - the schedule (every other Monday, for stays that have started);
  - the Rs 1,000 minimum and the 3% processing charge, kept on refunds;
  - refunds, chargebacks and negative balances; a bank account in the owner's own name;
  - the 72 hour hold after a change; withholding for suspected fraud; records; tax; ending payouts.
  - It opens with "StayKey payouts aren't available yet."
- **`/delete-account`:**
  - how to delete in the app (Profile, Delete account, 30 days to change your mind);
  - what is deleted and what is kept, and that a store subscription must be cancelled in the store first;
  - a mailto link for anyone who can't open the app, with the phone number to include.
- **`/download`:** a route handler that redirects (307) using `storeFor`.
- `not-found.tsx` in the site's style, with links home and to support.

### 3. Metadata
- Per page: `title` and `description`, a canonical URL (`alternates.canonical`), Open Graph and Twitter cards.
- `robots.ts`: allow everything, disallow `/billing`, `/join/`, `/verify-email` and `/download`, and point to the sitemap.
- `sitemap.ts`: `/`, `/pricing`, `/support`, `/privacy`, `/terms` and `/delete-account`, each with `lastModified` from its content date.
- `opengraph-image.tsx` at the root (the wordmark, the headline and a "Built for Sri Lanka" chip), plus one for `/pricing` with the Villa plan's price. Each has alt text.
- Icons from the app's icon: `icon.png`, `apple-icon.png` and the existing `favicon.ico`.
- JSON-LD on `/`: `Organization` (name, url, logo, support email) and `SoftwareApplication`:
  - `applicationCategory: BusinessApplication` and `operatingSystem: iOS, Android`;
  - `offers` from the plans, when they loaded.
  - Leave out FAQ markup, since Google no longer shows it for sites like this.
- `itunes: { appId }` in the root metadata when `STAYKEY_APP_STORE_ID` is set.
- `public/llms.txt`: what StayKey is, who it's for, prices and where the support and legal pages are, for AI search tools.
- `next.config.ts`:
  - redirect `www.staykey.direct` to the apex;
  - send `X-Content-Type-Options: nosniff`, `Referrer-Policy: strict-origin-when-cross-origin` and `Permissions-Policy` (camera, microphone, geolocation off) on every page.

### 4. Booking app
- A "Privacy" link to `https://staykey.direct/privacy` under the guest form on the checkout, and in the stay page's footer. The text: "StayKey handles your details for [property name]."

### 5. Mobile app
- `help.tsx`:
  - replace the PayHere topic with Payments.lk card payments;
  - take the WhatsApp number from `EXPO_PUBLIC_SUPPORT_WHATSAPP` and hide the row when it's unset;
  - add links to the support page, terms, privacy and account deletion.
- `paywall.tsx` and `subscription.tsx`: "Terms of use" and "Privacy policy" links under the purchase button, which App Store review requires for subscriptions.
- The onboarding phone step: "By continuing you agree to StayKey's Terms and Privacy Policy", with both links.
- `payout-details.tsx`: "Read the payout terms" opens `/terms/payouts`.
- `src/lib/links.ts` holds the site's URLs (`https://staykey.direct/...`), opened with `expo-web-browser`, which is already installed.

### 6. Docs
- `docs/ROADMAP.md`: tick the items, and add "App Store and Google Play listing links on the site" under launch readiness.
- `docs/HANDOVER.md`: the marketing site map, the calls above, gotchas, how to capture screenshots again, the gate status and the next step. Update `README.md` and the status line in `docs/backend-plan.md`.

## Do not

- Mention StayKey payouts on any public page while they are off in production, or index `/terms/payouts`.
- Add analytics, trackers, chat widgets or third-party scripts.
- Copy Apple's or Google's badge artwork by hand; use the official files once the listings exist.
- Fetch the API at build time, or let the page fail when the plans can't be loaded.
- Publish real legal text as reviewed before `LEGAL_REVIEWED` is set by a person.
- Add a contact form or collect anything on public pages.
- Change the billing, join or email confirmation flows.

## Validate

- `bunx biome check . && bun run typecheck`; `cd apps/marketing && bun test src && bun run build`; `cd apps/booking && bun run build`; `cd apps/mobile && bun test src && bunx expo export --platform ios`.
- With the API stopped, `next start` still serves `/` and `/pricing`, with "See prices in the app".
- With the seeded API running, check each of these:
  - `/pricing` shows Rs 2,900, Rs 5,900 and Rs 11,900 a month, and the yearly link switches the prices;
  - `/download` with an iPhone user agent redirects to `STAYKEY_APP_STORE_URL`, with an Android agent to Google Play, and with a desktop agent to `/#download`;
  - unset store links show "Coming soon".
- `curl` `/robots.txt`, `/sitemap.xml`, `/opengraph-image` and `/llms.txt`. The sitemap lists only the six public pages. A `<meta name="robots" content="noindex">` shows on billing, join, verify-email, `/terms/payouts` and, while drafts, on the legal pages.
- Playwright at 360, 768 and 1280 wide: screenshots of every page; tab through `/` and check each link and button can be reached with visible focus; no horizontal scroll.
- Lighthouse on mobile for `/` and `/pricing`: SEO 100, accessibility at least 95, best practices at least 95, performance at least 90. Then run the `next-pro-seo` audit and fix what it reports.
- The structured data passes schema.org's validator (paste the rendered JSON-LD).
- The booking page's checkout and stay page show the privacy link. The app's help, paywall, onboarding phone step and payout details open the right pages in its web build.
- **Gate:** a visitor finds the price on `/pricing`, reaches the App Store and Google Play from `/download` (once the listings exist), and reaches support from every page's footer.

## Risks

- **Legal text read as legal advice.** The drafts are marked as drafts until reviewed, and the company details come from the owner.
- **Prices drifting** between the site, the app's store prices and `domain.Plans`. The site reads web prices from the API. Store prices are only described ("the App Store shows the price in your currency"), never hard-coded on the site.
- **Store review.** Apple wants a privacy policy, terms of use links in the app and in the listing, and a support page. Google wants an account deletion page. This phase delivers all four before submission.
- **Overpromising.** Features name only what is built and switched on, and screenshots come from the real app.

## Went-sideways flags

- A client component or a third-party script on a public page.
- A build that fails because the API isn't running.
- StayKey payouts mentioned on a public page, or `/terms/payouts` in the sitemap.
- Raw hex colours or pixel values in components instead of tokens.
- A page with horizontal scroll at 360px, or a link without visible focus.

## Open questions

| Question | Default if unanswered |
| --- | --- |
| The legal entity behind StayKey: name, registered address, registration number and a contact for data protection | Placeholders in `company.ts`, legal pages marked as drafts; needed before launch |
| Support WhatsApp number | Email only until one is given |
| App Store and Google Play listing links and the App Store app id | "Coming soon" buttons until the listings exist |
| Analytics on the marketing site | None |
| Languages | English only |
| Show the app's store prices in dollars beside the rupee web prices | No; rupee web prices, and a note that the stores show their own price |
| A "Powered by StayKey" link on booking pages | No |
| Lawyer to review the terms, privacy policy and payout terms against the PDPA | Ask before launch; drafts meanwhile |
