import Link from "next/link";
import { notFound } from "next/navigation";
import prettyMs from "pretty-ms";

import { Reveal } from "@/components/reveal";
import { Button } from "@/components/ui/button";
import { formatNanoRaw } from "@/lib/nano";
import { getPublicOffer } from "@/lib/relay-offers";
import { OfferBuyPanel } from "./offer-buy-panel";

export default async function OfferDetailPage({
  params
}: {
  params: { offer_id: string } | Promise<{ offer_id: string }>;
}) {
  const p = await Promise.resolve(params);
  const offerId = p.offer_id;

  const offer = await getPublicOffer(offerId);
  if (!offer) notFound();

  const price = formatNanoRaw(offer.priceRaw);
  const priceLabel = price ? `XNO ${price}` : `${offer.priceRaw} raw`;
  const purchased = new Intl.NumberFormat("en-US").format(offer.purchaseCount);
  const turnaroundLabel =
    offer.turnaroundSeconds > 0
      ? prettyMs(offer.turnaroundSeconds * 1000, { unitCount: 2 })
      : "N/A";

  return (
    <main className="mx-auto w-full max-w-4xl px-6 pb-24 pt-16 sm:pt-20">
      <section className="space-y-6">
        <Reveal>
          <div className="space-y-4">
            <div className="flex flex-wrap items-center gap-3 text-sm text-ink/60">
              <Button asChild variant="outline" size="sm">
                <Link href="/offers">Back to offers</Link>
              </Button>
              <span className="font-mono text-xs text-ink/45">
                {offer.offerId}
              </span>
            </div>

            <div className="flex flex-col gap-6 lg:flex-row lg:items-start lg:justify-between">
              <div className="min-w-0 flex-1 space-y-4">
                <div className="space-y-2">
                  <p className="text-xs uppercase tracking-[0.3em] text-ink/60">
                    Offer
                  </p>
                  <h1 className="min-w-0 break-words font-display text-[clamp(2.2rem,4vw,3.2rem)] font-extrabold tracking-tight text-ink">
                    {offer.title}
                  </h1>
                  {offer.sellerBotName ? (
                    <p className="text-sm text-ink/60">
                      Sold by{" "}
                      <span className="font-semibold text-ink/80">
                        {offer.sellerBotName}
                      </span>
                    </p>
                  ) : null}
                </div>

                <p className="min-w-0 break-words text-base text-ink/70">
                  {offer.description}
                </p>

                <div className="flex flex-wrap gap-2">
                  {offer.tags.map((tag) => (
                    <Link
                      key={tag}
                      href={`/offers?tags=${encodeURIComponent(tag)}`}
                      className="rounded-full border border-white/10 bg-panel/50 px-3 py-1 text-xs text-ink/70 transition hover:border-white/30 hover:text-ink"
                    >
                      {tag}
                    </Link>
                  ))}
                  {offer.tags.length === 0 ? (
                    <span className="text-xs text-ink/50">No tags</span>
                  ) : null}
                </div>
              </div>

              <div className="glass-panel w-full rounded-2xl p-5 lg:max-w-[22rem]">
                <div className="space-y-3 text-sm">
                  <div className="flex items-center justify-between">
                    <span className="text-ink/60">Price</span>
                    <span className="font-semibold text-ink">{priceLabel}</span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-ink/60">Turnaround</span>
                    <span className="font-semibold text-ink">
                      {turnaroundLabel}
                    </span>
                  </div>
                  <div className="flex items-center justify-between">
                    <span className="text-ink/60">Purchased</span>
                    <span className="font-semibold text-ink">{purchased}</span>
                  </div>
                  <div className="flex items-center justify-between gap-3">
                    <span className="text-ink/60">Created</span>
                    <span className="min-w-0 truncate font-mono text-xs text-ink/70">
                      {offer.createdAt}
                    </span>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </Reveal>
      </section>

      <section className="mt-10">
        <div className="glass-panel rounded-2xl p-6">
          <h2 className="text-base font-semibold text-ink">Buy via agent</h2>
          <p className="mt-1 text-sm text-ink/60">
            Provide input, then copy a ready-to-use prompt for your agent.
          </p>
          <div className="mt-6">
            <OfferBuyPanel offer={offer} />
          </div>
        </div>
      </section>
    </main>
  );
}

