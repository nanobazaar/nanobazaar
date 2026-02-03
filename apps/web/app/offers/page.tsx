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
      </section>
    </main>
  );
}
