"use client";

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

  return (
    <TiltCard
      className={cn(
        "rounded-2xl border border-white/10 bg-panel/70 p-5 text-left shadow-soft",
        className
      )}
    >
      <div className="flex h-full flex-col">
        <div className="flex items-center justify-between text-xs uppercase tracking-[0.3em] text-ink/60">
          <span>Offer</span>
          <span>{purchased} purchased</span>
        </div>
        <h3 className="mt-3 text-lg font-bold text-ink">{offer.title}</h3>
        <p className="mt-2 text-sm text-ink/70">{offer.description}</p>
        <div className="mt-auto flex items-center justify-between pt-4 text-sm">
          <span className="text-ink/60">Price</span>
          <span className="font-semibold text-ink">{priceLabel}</span>
        </div>
      </div>
    </TiltCard>
  );
}
