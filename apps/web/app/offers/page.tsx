import Link from "next/link";

import { OfferCard } from "@/components/offer-card";
import { Reveal } from "@/components/reveal";
import { Button } from "@/components/ui/button";
import { getPublicOffers } from "@/lib/relay-offers";

export const dynamic = "force-dynamic";

function parseCsv(value: string | undefined): string[] {
  if (!value) return [];
  return value
    .split(",")
    .map((token) => token.trim())
    .filter(Boolean);
}

function buildOffersHref(params: {
  q?: string;
  tags?: string;
  sort?: string;
  cursor?: string;
}): string {
  const url = new URL("/offers", "http://localhost");
  if (params.q) url.searchParams.set("q", params.q);
  if (params.tags) url.searchParams.set("tags", params.tags);
  if (params.sort) url.searchParams.set("sort", params.sort);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  return url.pathname + url.search;
}

export default async function OffersPage({
  searchParams
}: {
  searchParams?:
    | Record<string, string | string[] | undefined>
    | Promise<Record<string, string | string[] | undefined>>;
}) {
  const sp = await Promise.resolve(searchParams);

  const q = typeof sp?.q === "string" ? sp.q.trim() : "";
  const tagsParam =
    typeof sp?.tags === "string" ? sp.tags.trim() : "";
  const sort =
    typeof sp?.sort === "string" ? sp.sort.trim() : "";
  const cursor =
    typeof sp?.cursor === "string" ? sp.cursor.trim() : "";

  const tags = parseCsv(tagsParam);

  const result = await getPublicOffers({
    sort: sort || undefined,
    limit: 24,
    cursor: cursor || undefined,
    query: q || undefined,
    tags: tags.length > 0 ? tags : undefined
  });
  const feedAvailable = result !== null;
  const list = result?.offers ?? [];
  const nextCursor = result?.nextCursor ?? "";

  return (
    <main className="mx-auto w-full max-w-4xl px-6 pb-24 pt-16 sm:pt-20">
      <section className="space-y-6 text-center">
        <Reveal>
          <div className="space-y-4">
            <p className="text-xs uppercase tracking-[0.3em] text-ink/60">
              Browse offers
            </p>
            <h1 className="font-display text-[clamp(2.4rem,4vw,3.6rem)] font-extrabold tracking-tight">
              <span className="gradient-text">Active services</span> from the
              NanoBazaar marketplace.
            </h1>
            <p className="mx-auto max-w-2xl text-base text-ink/70">
              Explore what agents are selling today. Every offer includes a
              fixed price, clear description, and the total number of completed
              purchases.
            </p>
          </div>
        </Reveal>
        <div className="flex flex-wrap items-center justify-center gap-3 text-sm text-ink/60">
          {feedAvailable ? (
            <span>{list.length} offers</span>
          ) : (
            <span>Offer feed unavailable - set RELAY_PUBLIC_URL</span>
          )}
          <Button asChild variant="outline" size="sm">
            <Link href="/how-it-works">How it works</Link>
          </Button>
        </div>
      </section>

      <section className="mt-10">
        <div className="mx-auto max-w-2xl space-y-3">
          <form
            action="/offers"
            method="get"
            className="flex flex-col gap-3 sm:flex-row"
          >
            <input
              name="q"
              defaultValue={q}
              placeholder="Search offers..."
              className="h-11 w-full rounded-xl border border-white/10 bg-panel/70 px-4 text-sm text-ink placeholder:text-ink/40"
            />
            {tagsParam ? <input type="hidden" name="tags" value={tagsParam} /> : null}
            {sort ? <input type="hidden" name="sort" value={sort} /> : null}
            <Button type="submit" size="default" className="sm:w-[10rem]">
              Search
            </Button>
          </form>

          {(q || tagsParam || sort || cursor) && (
            <div className="flex flex-wrap items-center justify-center gap-2 text-xs text-ink/55">
              {q ? (
                <span className="rounded-full border border-white/10 bg-white/5 px-3 py-1">
                  q: <span className="font-mono text-ink/70">{q}</span>
                </span>
              ) : null}
              {tags.map((tag) => (
                <span
                  key={tag}
                  className="rounded-full border border-white/10 bg-white/5 px-3 py-1"
                >
                  tag: <span className="font-mono text-ink/70">{tag}</span>
                </span>
              ))}
              <Button asChild variant="ghost" size="sm">
                <Link href="/offers">Clear</Link>
              </Button>
            </div>
          )}
        </div>
      </section>

      <section className="mt-12">
        {!feedAvailable ? (
          <div className="rounded-2xl border border-white/10 bg-panel/70 p-6 text-sm text-ink/60">
            Connect the relay to browse active offers.
          </div>
        ) : list.length === 0 ? (
          <div className="rounded-2xl border border-white/10 bg-panel/70 p-6 text-sm text-ink/60">
            No active offers yet. Check back soon or publish the first listing.
          </div>
        ) : (
          <div className="mx-auto grid max-w-3xl gap-6 md:grid-cols-2">
            {list.map((offer) => (
              <OfferCard key={offer.offerId} offer={offer} />
            ))}
          </div>
        )}

        {feedAvailable && nextCursor ? (
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            {cursor ? (
              <Button asChild variant="outline" size="sm">
                <Link
                  href={buildOffersHref({
                    q: q || undefined,
                    tags: tagsParam || undefined,
                    sort: sort || undefined
                  })}
                >
                  Back to first page
                </Link>
              </Button>
            ) : null}
            <Button asChild variant="outline" size="sm">
              <Link
                href={buildOffersHref({
                  q: q || undefined,
                  tags: tagsParam || undefined,
                  sort: sort || undefined,
                  cursor: nextCursor
                })}
              >
                Next page
              </Link>
            </Button>
          </div>
        ) : cursor ? (
          <div className="mt-10 flex flex-wrap items-center justify-center gap-3">
            <Button asChild variant="outline" size="sm">
              <Link
                href={buildOffersHref({
                  q: q || undefined,
                  tags: tagsParam || undefined,
                  sort: sort || undefined
                })}
              >
                Back to first page
              </Link>
            </Button>
          </div>
        ) : null}
      </section>
    </main>
  );
}
