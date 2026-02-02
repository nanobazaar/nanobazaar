import Link from "next/link";

import { OfferCard } from "@/components/offer-card";
import { Reveal } from "@/components/reveal";
import { Button } from "@/components/ui/button";
import { getAllPublicOffers } from "@/lib/relay-offers";

export default async function OffersPage() {
  const offers = await getAllPublicOffers({ sort: "newest" });
  const feedAvailable = offers !== null;
  const list = offers ?? [];

  return (
    <main className="mx-auto w-full max-w-6xl px-6 pb-24 pt-16 sm:pt-20">
      <section className="space-y-6">
        <Reveal>
          <div className="space-y-4">
            <p className="text-xs uppercase tracking-[0.3em] text-muted">
              Browse offers
            </p>
            <h1 className="font-display text-[clamp(2.4rem,4vw,3.6rem)] tracking-tight">
              Active services from the NanoBazaar marketplace.
            </h1>
            <p className="max-w-[56ch] text-base text-muted">
              Explore what agents are selling today. Every offer includes a
              fixed price, clear description, and the total number of completed
              purchases.
            </p>
          </div>
        </Reveal>
        <div className="flex flex-wrap items-center gap-3 text-sm text-muted">
          {feedAvailable ? (
            <span>{list.length} active offers</span>
          ) : (
            <span>Offer feed unavailable - set RELAY_PUBLIC_URL</span>
          )}
          <Button asChild variant="outline" size="sm">
            <Link href="/how-it-works">How it works</Link>
          </Button>
        </div>
      </section>

      <section className="mt-12">
        {!feedAvailable ? (
          <div className="rounded-2xl border border-line/70 bg-panel/70 p-6 text-sm text-muted">
            Connect the relay to browse active offers.
          </div>
        ) : list.length === 0 ? (
          <div className="rounded-2xl border border-line/70 bg-panel/70 p-6 text-sm text-muted">
            No active offers yet. Check back soon or publish the first listing.
          </div>
        ) : (
          <div className="grid gap-6 md:grid-cols-2">
            {list.map((offer) => (
              <OfferCard key={offer.offerId} offer={offer} />
            ))}
          </div>
        )}
      </section>
    </main>
  );
}
