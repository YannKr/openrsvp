# Design System — OpenRSVP

## Product Context
- **What this is:** Self-hosted, open-source RSVP and event invitation platform — the open-source alternative to Evite, Partiful, and Luma.
- **Who it's for:** Event organizers (parents planning birthday parties, community organizers running meetups, individuals hosting gatherings), their guests, and self-hosting enthusiasts deploying the platform.
- **Space/industry:** Event invitation and RSVP management. Peers: Luma, Partiful, Paperless Post, RSVPify, EventCreate.
- **Project type:** Web app (SvelteKit frontend, Go backend) with public-facing event pages and authenticated organizer/admin dashboards.

## Aesthetic Direction
- **Direction:** Refined Warmth — Clean and professional like Luma, with enough warmth and personality to feel celebratory. Not corporate SaaS, not cartoonish. The feeling: "the friend who always throws great parties and has their shit together."
- **Decoration level:** Intentional — Subtle gradient backgrounds on key pages (extending landing page DNA), frosted glass card effects for depth. Not gradient-heavy everywhere (that's Partiful's territory).
- **Mood:** Warm, inviting, trustworthy. A parent planning their child's birthday party should feel confident sharing this link. A dev evaluating self-hosted tools should see a product, not a hobby project.
- **Reference sites:** lu.ma (clean, professional, vibrant), partiful.com (social, warm), paperlesspost.com (elegant)

## Typography
- **Display/Hero:** Satoshi (Fontshare) — Geometric sans with genuine personality. More distinctive than Inter, not quirky. Used for headings, event titles, hero text, stat values.
- **Body:** Plus Jakarta Sans (Google Fonts) — Warm, rounded, highly readable. Complements Satoshi's geometry with a friendlier voice. Used for paragraphs, form labels, descriptions, navigation.
- **UI/Labels:** Same as body (Plus Jakarta Sans) at smaller weights.
- **Data/Tables:** Geist Mono (Fontsource) — Clean monospace with tabular numbers. Used for timestamps, IDs, statistics, and code.
- **Code:** Geist Mono
- **Loading:** Google Fonts for Plus Jakarta Sans, Fontshare API for Satoshi, Fontsource for Geist Mono. Self-hosted copies bundled in Docker image for offline/air-gapped deployments.
- **Scale:**
  - `text-xs`: 12px / 0.75rem
  - `text-sm`: 14px / 0.875rem
  - `text-base`: 16px / 1rem
  - `text-lg`: 18px / 1.125rem
  - `text-xl`: 20px / 1.25rem
  - `text-2xl`: 24px / 1.5rem
  - `text-3xl`: 30px / 1.875rem
  - `text-4xl`: 36px / 2.25rem
  - `text-5xl`: 48px / 3rem
  - `text-6xl`: 56px / 3.5rem (hero only)

## Color
- **Approach:** Balanced — warm rose primary for celebration and brand identity, clean blue secondary for interactive elements, warm stone neutrals throughout.
- **Primary:** `#E54666` — Warm rose. Celebratory, inviting, distinctive. Used for CTAs, brand accents, active states, focus rings.
  - Hover: `#D63D5C`
  - Light: `#FDE8EC` (backgrounds, badges)
  - Lighter: `#FFF1F3` (subtle fills)
- **Secondary:** `#2563EB` (blue-600) — Links, interactive elements, informational highlights.
  - Hover: `#1D4ED8`
  - Light: `#DBEAFE`
- **Neutrals:** Stone family (warm grays, not cool slate):
  - 50: `#FAFAF9` — Page backgrounds
  - 100: `#F5F5F4` — Card backgrounds, subtle fills
  - 200: `#E7E5E4` — Borders, dividers
  - 300: `#D6D3D1` — Hover borders
  - 400: `#A8A29E` — Muted text, placeholders
  - 500: `#78716C` — Secondary text
  - 600: `#57534E` — Heavy secondary text
  - 700: `#44403C` — Primary text (body)
  - 800: `#292524` — Emphasized text
  - 900: `#1C1917` — Headings, display text
  - 950: `#0C0A09` — Dark mode backgrounds
- **Semantic:**
  - Success: `#16A34A` / light: `#DCFCE7`
  - Warning: `#D97706` / light: `#FEF3C7`
  - Error: `#DC2626` / light: `#FEE2E2`
  - Info: `#2563EB` / light: `#DBEAFE`
- **Dark mode:** Invert surfaces (stone-950 base, stone-900 cards), desaturate primary to `#F06B85`, increase neutral text to stone-50/stone-100. Reduce shadow intensity, increase opacity. Full dark palette defined in CSS custom properties.

## Spacing
- **Base unit:** 4px
- **Density:** Comfortable — breathing room without waste. Event pages and forms should feel spacious; dashboards can be slightly tighter.
- **Scale:**
  - `2xs`: 2px
  - `xs`: 4px
  - `sm`: 8px
  - `md`: 16px
  - `lg`: 24px
  - `xl`: 32px
  - `2xl`: 48px
  - `3xl`: 64px

## Layout
- **Approach:** Grid-disciplined — Strict grid for app pages (dashboard, admin, event management), slightly looser for public event pages (centered single-column with generous whitespace).
- **Grid:** 1 column mobile, 2 columns tablet, 3-4 columns desktop for card layouts. Single centered column (max 640px) for event pages and forms.
- **Max content width:** 72rem (1152px) — Narrower than default 80rem for better readability and focus.
- **Border radius:** Hierarchical scale for warmth:
  - `sm`: 6px — Small elements (badges, chips, inline tags)
  - `md`: 10px — Default (buttons, inputs, small cards)
  - `lg`: 14px — Medium elements (cards, modals, dropdowns)
  - `xl`: 20px — Large containers (page sections, feature cards)
  - `full`: 9999px — Pills, avatars, toggle switches

## Shadows
- **sm:** `0 1px 2px 0 rgba(28, 25, 23, 0.04)` — Subtle elevation for cards at rest
- **md:** `0 4px 6px -1px rgba(28, 25, 23, 0.06), 0 2px 4px -2px rgba(28, 25, 23, 0.06)` — Hover states, dropdowns
- **lg:** `0 10px 15px -3px rgba(28, 25, 23, 0.07), 0 4px 6px -4px rgba(28, 25, 23, 0.07)` — Modals, popovers
- **xl:** `0 20px 25px -5px rgba(28, 25, 23, 0.08), 0 8px 10px -6px rgba(28, 25, 23, 0.08)` — Featured elements, mockup frames
- Note: Shadow color uses warm stone-900 base, not pure black, for cohesion with warm neutrals.

## Motion
- **Approach:** Intentional — Subtle entrance animations, meaningful hover states (shadow lift + slight translateY), smooth transitions. Not springy/bouncy (too playful for self-hosting context). Every animation must serve comprehension or feedback.
- **Easing:**
  - Enter: `cubic-bezier(0.16, 1, 0.3, 1)` (ease-out — fast start, gentle settle)
  - Exit: `cubic-bezier(0.7, 0, 0.84, 0)` (ease-in — gentle start, fast exit)
  - Move: `cubic-bezier(0.45, 0, 0.55, 1)` (ease-in-out — smooth repositioning)
- **Duration:**
  - Micro: 50-100ms (button press feedback, toggle switches)
  - Short: 150-200ms (hover states, focus rings, color transitions)
  - Medium: 250-350ms (dropdowns, modals entering, card hover lift)
  - Long: 400-700ms (page transitions, toast entrances, only when needed)
- **Hover pattern:** Cards and buttons lift on hover: `transform: translateY(-2px)` + shadow upgrade from sm→md. Transition: 200ms ease-out.

## Decoration Patterns
- **Gradient backgrounds:** Subtle radial gradients on landing page, event pages, and auth pages. Use primary-light and secondary-light at low opacity with blur. Not on dashboard/admin pages.
- **Frosted glass:** `backdrop-filter: blur(12px)` with semi-transparent backgrounds on sticky headers and overlay elements.
- **Focus rings:** `0 0 0 3px var(--primary-light)` on all interactive elements when focused.

## Decisions Log
| Date | Decision | Rationale |
|------|----------|-----------|
| 2026-03-18 | Initial design system created | Created by /design-consultation based on competitive research (Luma, Partiful, Paperless Post, EventCreate) and office-hours design doc. Warm rose chosen to differentiate from blue/purple convention in the RSVP space. Stone neutrals for warmth. Satoshi + Plus Jakarta Sans for personality without quirk. |
| 2026-03-18 | Parent-friendly audience priority | User specified design must appeal to parents. Warm palette, friendly typography, trustworthy feel all serve this. Event examples use birthday parties, BBQs, school events. |
| 2026-03-18 | Dark mode included | Full dark mode palette defined with desaturated primary and inverted surfaces. CSS custom properties enable toggle. |
