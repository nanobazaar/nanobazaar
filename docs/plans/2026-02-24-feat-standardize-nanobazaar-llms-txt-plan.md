---
title: "feat(web): Standardize NanoBazaar llms.txt"
type: feat
status: completed
date: 2026-02-24
---

# feat(web): Standardize NanoBazaar llms.txt

## Overview

NanoBazaar already serves `/llms.txt`, but the current file is minimal and not fully aligned with the `llmstxt.org` recommended structure (named markdown links grouped by section). This plan standardizes the file format, improves discoverability for LLM crawlers, and defines a lightweight maintenance path.

## Problem Statement / Motivation

- The current file (`apps/web/public/llms.txt`) is live but uses plain URL bullets, not structured markdown link entries.
- The site already signals `llms.txt` in multiple places (`<head>` alternate link and sitemap), so content quality is now the limiting factor.
- A more structured file should improve machine readability and reduce ambiguity about key pages and usage context.

## Consolidated Research

### Internal Repo Findings

- Existing file:
  - `apps/web/public/llms.txt:1` exists and is served in production.
- Existing discovery plumbing is already present:
  - `apps/web/app/layout.tsx:58` includes `<link rel="alternate" type="text/plain" href="/llms.txt" ... />`
  - `apps/web/app/sitemap.ts:11` includes `/llms.txt` in static routes.
- Framework/runtime context:
  - `apps/web/package.json:20` uses `next@^16.1.6`.
- Institutional learnings:
  - No `docs/solutions/` directory exists in this repo, so there are no prior institutional solution docs to reuse.

### External Findings

- `llmstxt.org` defines a canonical structure:
  - H1 project name
  - blockquote summary
  - optional markdown context
  - H2 sections containing markdown links with optional descriptions.
- The official llms-txt reference repository documents intent and common usage patterns, including optional `llms-full.txt`.
- Next.js 16 docs confirm:
  - static files in `public/` are served from root paths
  - custom text endpoints can also be generated via App Router Route Handlers when dynamic generation is needed.
- SiteSpeak generator documentation is useful for drafting candidate content quickly, but output should be reviewed against the canonical llmstxt structure.

### Research Decision

External research was included because this feature depends on a fast-moving, externally defined convention (`llms.txt`) and the user explicitly provided third-party documentation to follow.

## Stakeholders

- End users / agent operators: better machine-readable entry point to NanoBazaar docs and pages.
- Marketing/content maintainers: clear structure for what pages are indexed by LLM agents.
- Developers: low-complexity static content change with optional route/file extension (`llms-full.txt`).

## Proposed Solution

### 1. Standardize `llms.txt` Structure

- Rewrite `apps/web/public/llms.txt` to follow `llmstxt.org` formatting:
  - Title (`# NanoBazaar`)
  - One-sentence blockquote summary
  - Sectioned markdown links (no raw URL-only list)
  - Concise descriptions per link.

### 2. Curate High-Value Links

- Include core routes currently exposed in sitemap:
  - `/`
  - `/offers`
  - `/how-it-works`
  - `/faq`
  - `/troubleshooting`
- Keep `nanoargument.com` as related context, but present as explicit markdown links with rationale.

### 3. Decide on Optional `llms-full.txt`

- If long-form agent guidance is needed, add `apps/web/public/llms-full.txt` and link to it from `llms.txt`.
- If not needed immediately, defer and explicitly document that decision in the plan implementation notes.

### 4. Keep Discovery Paths Consistent

- Confirm existing references remain valid after content update:
  - `apps/web/app/layout.tsx`
  - `apps/web/app/sitemap.ts`
- If `llms-full.txt` is added, include it in `apps/web/app/sitemap.ts`.

### 5. Add Lightweight Maintenance Notes

- Add a short doc section (or comment block in plan handoff) describing who updates `llms.txt` when key routes or messaging change.

## Implementation Outcome (2026-02-24)

- Standardized `apps/web/public/llms.txt` to use sectioned markdown links and concise per-link descriptions.
- Kept existing discovery plumbing unchanged (`layout.tsx` alternate link and sitemap route entry were already correct).
- Explicitly deferred `llms-full.txt` in this iteration to keep scope small and maintenance overhead low.

## SpecFlow Analysis

### User Flow Overview

1. LLM crawler requests `https://nanobazaar.ai/llms.txt`.
2. Crawler parses summary and sections, then follows listed links.
3. Crawler optionally follows `llms-full.txt` for deeper context (if present).
4. Crawler uses linked content to build retrieval context for downstream Q&A/agent tasks.

### Flow Permutations Matrix

| Flow Variant | Entry Point | Expected Behavior |
| --- | --- | --- |
| Direct discovery | `/llms.txt` | 200 response, text file, parseable markdown structure |
| HTML-discovery | `<head>` alternate link | crawler can discover `/llms.txt` without hardcoded path |
| Sitemap-discovery | `/sitemap.xml` includes `/llms.txt` | crawler can discover via sitemap traversal |
| Deep-context | `/llms-full.txt` exists | crawler can choose compact vs full context |

### Missing Elements & Gaps

- **Content Governance**: no explicit owner/update trigger for `llms.txt`.
- **Format Consistency**: no validation guardrail (manual drift risk).
- **Scope Decision**: unclear whether to ship only `llms.txt` now or also `llms-full.txt`.

### Critical Questions Requiring Clarification

1. **Important**: Should `llms-full.txt` ship in this iteration, or is compact `llms.txt` sufficient?
   - Why it matters: affects scope, review size, and maintenance.
   - Default assumption if unanswered: ship only standardized `llms.txt`.
2. **Important**: Should `llms.txt` include only public marketing pages, or also operational docs (CLI/skill docs) from the monorepo?
   - Why it matters: impacts crawler context quality and noise.
   - Default assumption if unanswered: include public web pages + one related context link.

## Technical Considerations

- Keep this as a static file in `public/` unless dynamic generation is explicitly required.
- Avoid adding dependencies; this is primarily content + route list consistency.
- Ensure no broken links and keep descriptions concise (crawler-friendly).
- If generated by tools (e.g., SiteSpeak), apply manual review before commit.

## Acceptance Criteria

- [x] `apps/web/public/llms.txt` uses `llmstxt.org`-aligned structure (H1, blockquote, sectioned markdown links).
- [x] `apps/web/public/llms.txt` includes all canonical public NanoBazaar routes currently intended for crawler discovery.
- [x] `apps/web/app/layout.tsx` continues advertising `/llms.txt` in `<head>`.
- [x] `apps/web/app/sitemap.ts` includes `/llms.txt` and also `/llms-full.txt` if that file is added.
- [x] `apps/web/public/llms-full.txt` decision is explicit (implemented or deferred with rationale).
- [x] Link check passes for all URLs listed in `apps/web/public/llms.txt`.

## Success Metrics

- `https://nanobazaar.ai/llms.txt` remains reachable and reflects agreed canonical pages.
- Internal reviewers confirm file readability and alignment with llms-txt conventions.
- No regressions in existing metadata/sitemap behavior.

## Dependencies & Risks

### Dependencies

- Current public page inventory in `apps/web/app/`.
- Agreement on whether to include `llms-full.txt`.

### Risks

- Overly long or noisy content may reduce crawler usefulness.
- Manual edits can drift from canonical page set over time.

## Implementation Sketch (MVP)

### File Touch List

- `apps/web/public/llms.txt`
- `apps/web/public/llms-full.txt` (optional)
- `apps/web/app/sitemap.ts` (conditional, if `llms-full.txt` is added)

### Pseudocode Example

```markdown
# apps/web/public/llms.txt
# NanoBazaar
> Public relay where agents sell services with encrypted payloads and Nano settlement.

## Core Pages
- [Homepage](https://nanobazaar.ai/): Product overview and positioning.
- [Offers](https://nanobazaar.ai/offers): Live marketplace listings.
- [How It Works](https://nanobazaar.ai/how-it-works): End-to-end flow.
- [FAQ](https://nanobazaar.ai/faq): Operational Q&A.
- [Troubleshooting](https://nanobazaar.ai/troubleshooting): Common fixes.

## Related Context
- [Nano Argument](https://nanoargument.com/): Rationale for Nano as payment rail.
```

## AI-Era Considerations

- AI-assisted generators can draft `llms.txt`, but human review is required for factual accuracy and page prioritization.
- Keep acceptance criteria explicit to avoid “looks good” but structurally invalid output.

## References & Research

### Internal References

- `apps/web/public/llms.txt:1`
- `apps/web/app/layout.tsx:58`
- `apps/web/app/sitemap.ts:5`
- `apps/web/package.json:20`

### External References

- llms.txt canonical format: https://llmstxt.org/
- llms-txt reference repository: https://github.com/AnswerDotAI/llms-txt
- SiteSpeak generator guide: https://sitespeak.ai/tools/llms-txt-generator
- Next.js 16 public folder docs: https://github.com/vercel/next.js/blob/v16.1.6/docs/01-app/03-api-reference/03-file-conventions/public-folder.mdx
- Next.js 16 route handlers + custom text endpoints: https://github.com/vercel/next.js/blob/v16.1.6/docs/01-app/02-guides/backend-for-frontend.mdx

## Final Review Checklist

- [x] Title is descriptive and searchable.
- [x] Acceptance criteria are measurable.
- [x] File names are explicit in todo/checklist and pseudocode.
- [x] No contract artifacts (`CONTRACT.md`, `OPENAPI.yaml`, `TEST_VECTORS.md`) are modified.
