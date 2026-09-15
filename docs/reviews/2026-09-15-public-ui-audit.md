# NanoBazaar public UI audit — 2026-09-15

Scope: live nanobazaar.ai, then local fixes on codex/durable-agent-commerce. Preserve prior uncommitted commerce and seller-contact changes. No production writes, payment, release or deployment.

## Confirmed live findings

| Area | Evidence | Correction |
| --- | --- | --- |
| Home and offers | At 375 px, page content reaches 464 px. Global offer IDs force cards wider than their grid. Desktop card footers also clip. | Remove IDs from cards, constrain grid children, wrap long seller content. |
| Offer detail | Long ID overflows the header; nanosecond creation date is truncated; full JSON dominates the purchase panel. | Show readable metadata, keep exact IDs in agent data and copied instructions. |
| FAQ | Content reaches 502 px at 375 px due to state paths and code in grid cards. | Allow cards to shrink and wrap code without dropping content. |
| Troubleshooting | Content reaches 642 px at 375 px due to recovery commands in cards. | Same shared layout correction and responsive callout padding. |
| How it works | Install text area clips its instruction (85 px content in 72 px). | Readable, selectable installation instructions with an honest copy result. |
| Header | Full desktop navigation compresses the logo at tablet width. | Keep compact navigation until the full menu fits. |
| Agent entry | Large hero logo pushes actions below the mobile fold. Agent instructions exist only as a head link; setup assumes OpenClaw. | Lead with browsing and agent instructions; runtime-neutral CLI path, optional OpenClaw integration. |
| Purchase handoff | “Buy via agent” only copies text. Empty buyer input silently becomes the seller's input guidance. | Explicit copy label and visible prompt; no invented buyer input; separate seller data from instructions. |
| Availability | “Agents online” counts all non-revoked registrations; “Jobs completed” includes PAID jobs. | Accurate user-facing labels without changing the API. |
| Error/empty states | Public UI exposes server configuration variables; empty search claims there are no offers. | Actionable visitor-facing errors and search-specific empty state. |
| Scrolling | A browser-agent click immediately after smooth scrolling could miss its target while the page was still moving. | Remove automatic smooth scrolling and keep stable anchor offsets. |
| Rendering | Content starts with inline opacity 0 until viewport animation runs; animated statistics initially render 0. | Show authoritative content in initial HTML, including for agents without JavaScript. |

## Validation

- Read-only live audit of `/`, `/offers`, offer detail, `/how-it-works`, `/faq`, and `/troubleshooting`. Authenticated admin screens were outside scope.
- Local Chromium checks of all six page types at 320, 375, 820 and 1440 px: no horizontal overflow across 24 combinations. Browser scrollbars account for a 15 px difference between viewport and document client width.
- Used all seven real public offers fetched on 15 September, then separate fixtures for current/old/unknown seller contact, 80-character unbroken titles, 64-character seller names, long URLs/input guidance and exact 39-digit raw prices. Synthetic values were served only from a local read-only fixture server.
- Verified search submission, no-match and unavailable-feed messages, and a filtered JSON link returning the same result set. Single-line search inputs scroll their contents normally; they do not widen the page.
- Verified navigation from an offer card, native offer-data disclosure, exact JSON identifiers/input guidance, mobile menu opening and Escape returning focus to its trigger.
- Verified empty task input stays null, multiline input survives unchanged, and copy success/failure states. Browser clipboard responses were controlled locally to exercise both outcomes; no real order or payment was created.
- Inspected initial server HTML: offer content, real fixture statistics, agent links and readable instructions are present without JavaScript or viewport animations. `/llms.txt` is directly fetchable.
- `pnpm lint`, `pnpm test:unit` (4 passing tests, Node 24.12.0), `pnpm build`, and `git diff --check` passed. Build reports the existing stale Browserslist-data warning.
- Frozen contract artifacts remain unchanged. Prior commerce/presence work is preserved. No production dependencies were added.

## Visual evidence

- [Live mobile before](/Users/madsbjerre/.codex/visualizations/2026/09/14/01a0a1d9-42ed-7a01-bb42-ab051a1bcc4e/nanobazaar-offers-before.png)
- [Local mobile after](/Users/madsbjerre/.codex/visualizations/2026/09/14/01a0a1d9-42ed-7a01-bb42-ab051a1bcc4e/nanobazaar-offers-after-mobile.png)
- [Local desktop after](/Users/madsbjerre/.codex/visualizations/2026/09/14/01a0a1d9-42ed-7a01-bb42-ab051a1bcc4e/nanobazaar-offers-after-desktop.png)

The after screenshots contain synthetic seller-contact timestamps for layout verification. They are not evidence of live seller availability. Changes are local and have not been deployed.
