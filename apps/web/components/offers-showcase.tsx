"use client";

import * as React from "react";
import { Button } from "@/components/ui/button";
import { OfferCard } from "@/components/offer-card";
import type { PublicOffer } from "@/lib/relay-offers";

type OffersShowcaseProps = {
  latestOffers: PublicOffer[];
  topOffers: PublicOffer[];
  feedAvailable: boolean;
};

type OfferView = "latest" | "most";

export function OffersShowcase({
  latestOffers,
  topOffers,
  feedAvailable
}: OffersShowcaseProps) {
  const [view, setView] = React.useState<OfferView>("latest");
  const offers = view === "latest" ? latestOffers : topOffers;

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap gap-3">
        <Button
          type="button"
          size="sm"
          variant={view === "latest" ? "default" : "outline"}
          aria-pressed={view === "latest"}
          onClick={() => setView("latest")}
        >
          Latest
        </Button>
        <Button
          type="button"
          size="sm"
          variant={view === "most" ? "default" : "outline"}
          aria-pressed={view === "most"}
          onClick={() => setView("most")}
        >
          Most purchased
        </Button>
      </div>

      {!feedAvailable ? (
        <div className="rounded-2xl border border-line/70 bg-panel/70 p-6 text-sm text-muted">
          Offer feed unavailable - set RELAY_PUBLIC_URL
        </div>
      ) : offers.length === 0 ? (
        <div className="rounded-2xl border border-line/70 bg-panel/70 p-6 text-sm text-muted">
          No offers to show yet.
        </div>
      ) : (
        <div className="grid gap-6 md:grid-cols-2">
          {offers.map((offer) => (
            <OfferCard key={offer.offerId} offer={offer} />
          ))}
        </div>
      )}
    </div>
  );
}
