import Link from "next/link";
import prettyMs from "pretty-ms";

import { SellerLastSeen } from "@/components/seller-last-seen";
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
  const turnaround = offer.turnaroundSeconds > 0
    ? prettyMs(offer.turnaroundSeconds * 1000, { unitCount: 2 })
    : "Not specified";

  return (
    <article className={cn(
      "relative flex min-w-0 flex-col rounded-2xl border border-white/10 bg-panel/70 p-5 text-left shadow-soft transition hover:border-white/30 focus-within:ring-2 focus-within:ring-accent/50 [overflow-wrap:anywhere]",
      className
    )}>
      <p className="text-xs text-ink/60">
        {new Intl.NumberFormat("en-US").format(offer.purchaseCount)} purchases
      </p>
      <h3 className="mt-3 text-lg font-bold leading-snug text-ink">
        <Link href={`/offers/${offer.offerId}`} className="after:absolute after:inset-0 after:rounded-2xl focus-visible:outline-none">
          {offer.title}
        </Link>
      </h3>
      {offer.sellerBotName ? <p className="mt-2 text-xs text-ink/65">By {offer.sellerBotName}</p> : null}
      <SellerLastSeen at={offer.sellerLastSeenAt} className="mt-2" />
      <p className="mt-3 line-clamp-4 text-sm leading-relaxed text-ink/70">{offer.description}</p>
      <dl className="mt-auto space-y-2 pt-5 text-sm">
        <div className="grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-3">
          <dt className="text-ink/60">Price</dt>
          <dd className="text-right font-semibold text-ink">{priceLabel}</dd>
        </div>
        <div className="grid grid-cols-[auto_minmax(0,1fr)] items-baseline gap-3">
          <dt className="text-ink/60">Turnaround</dt>
          <dd className="text-right font-semibold text-ink">{turnaround}</dd>
        </div>
      </dl>
      <span aria-hidden="true" className="mt-5 text-xs font-semibold text-accent">View offer →</span>
    </article>
  );
}
