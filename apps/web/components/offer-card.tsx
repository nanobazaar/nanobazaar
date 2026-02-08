"use client";

import Link from "next/link";
import prettyMs from "pretty-ms";

import { TiltCard } from "@/components/tilt-card";
import { cn } from "@/lib/utils";
import { formatNanoRaw } from "@/lib/nano";
import type { PublicOffer } from "@/lib/relay-offers";

type OfferCardProps = {
  offer: PublicOffer;
  className?: string;
};

export function OfferCard({ offer, className }: OfferCardProps) {
  const price = formatNanoRaw(offer.priceRaw);
  const priceLabel = price ? `XNO ${price}` : `${offer.priceRaw} raw`;
  const purchased = new Intl.NumberFormat("en-US").format(
    offer.purchaseCount
  );
  const turnaroundLabel =
    offer.turnaroundSeconds > 0
      ? prettyMs(offer.turnaroundSeconds * 1000, { unitCount: 2 })
      : "N/A";

  return (
    <Link
      href={`/offers/${offer.offerId}`}
      aria-label={`View offer ${offer.title}`}
      className="block rounded-2xl focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50"
    >
      <TiltCard
        className={cn(
          "min-w-0 overflow-hidden rounded-2xl border border-white/10 bg-panel/70 p-5 text-left shadow-soft transition hover:border-white/20",
          className
        )}
      >
        <div className="flex items-center justify-between text-xs uppercase tracking-[0.3em] text-ink/60">
          <span>Offer</span>
          <span>{purchased} purchased</span>
        </div>
        <h3 className="mt-3 min-w-0 break-words text-lg font-bold text-ink">
          {offer.title}
        </h3>
        {offer.sellerBotName ? (
          <p className="mt-1 text-xs text-ink/55">
            Seller:{" "}
            <span className="font-semibold text-ink/70">
              {offer.sellerBotName}
            </span>
          </p>
        ) : null}
        <p className="mt-2 min-w-0 break-words text-sm text-ink/70">
          {offer.description}
        </p>
        <div className="mt-5 space-y-2 text-sm">
          <div className="flex items-center justify-between">
            <span className="text-ink/60">Price</span>
            <span className="font-semibold text-ink">{priceLabel}</span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-ink/60">Turnaround</span>
            <span className="font-semibold text-ink">{turnaroundLabel}</span>
          </div>
        </div>
        <div className="mt-5 flex items-center justify-between gap-3 text-xs">
          <span className="font-mono text-ink/50">{offer.offerId}</span>
          <span className="text-[10px] font-semibold uppercase tracking-[0.22em] text-ink/55">
            View details
          </span>
        </div>
      </TiltCard>
    </Link>
  );
}
