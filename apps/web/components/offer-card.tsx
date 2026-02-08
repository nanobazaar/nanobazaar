"use client";

import * as React from "react";
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
  const [isFlipped, setIsFlipped] = React.useState(false);
  const [inputText, setInputText] = React.useState("");
  const [copyStatus, setCopyStatus] = React.useState<
    "idle" | "copied" | "error"
  >("idle");
  const price = formatNanoRaw(offer.priceRaw);
  const priceLabel = price ? `XNO ${price}` : `${offer.priceRaw} raw`;
  const purchased = new Intl.NumberFormat("en-US").format(
    offer.purchaseCount
  );
  const turnaroundLabel =
    offer.turnaroundSeconds > 0
      ? prettyMs(offer.turnaroundSeconds * 1000, { unitCount: 2 })
      : "N/A";
  const requestSchemaHint = offer.requestSchemaHint?.trim() ?? "";
  const guidanceText =
    requestSchemaHint || "No input guidance provided for this offer.";
  const promptInput =
    inputText.trim() || requestSchemaHint || "YOUR_INPUT_HERE";
  const offerJson = React.useMemo(
    () =>
      JSON.stringify(
        {
          offer_id: offer.offerId,
          ...(offer.sellerBotName ? { seller_bot_name: offer.sellerBotName } : {}),
          title: offer.title,
          description: offer.description,
          tags: offer.tags,
          price_raw: offer.priceRaw,
          turnaround_seconds: offer.turnaroundSeconds,
          purchase_count: offer.purchaseCount,
          created_at: offer.createdAt,
          ...(requestSchemaHint
            ? { request_schema_hint: requestSchemaHint }
            : {})
        },
        null,
        2
      ),
    [
      offer.createdAt,
      offer.description,
      offer.offerId,
      offer.priceRaw,
      offer.sellerBotName,
      offer.turnaroundSeconds,
      offer.purchaseCount,
      offer.tags,
      offer.title,
      requestSchemaHint
    ]
  );

  const handleCopyPrompt = async () => {
    if (!navigator?.clipboard?.writeText) {
      setCopyStatus("error");
      return;
    }
    try {
      await navigator.clipboard.writeText(
        `use the nanobazaar skill to buy offer with ID ${offer.offerId} and add this input: ${promptInput}`
      );
      setCopyStatus("copied");
      window.setTimeout(() => setCopyStatus("idle"), 2000);
    } catch {
      setCopyStatus("error");
    }
  };

  return (
    <TiltCard
      className={cn(
        "min-w-0 overflow-hidden rounded-2xl border border-white/10 bg-panel/70 p-5 text-left shadow-soft",
        className
      )}
    >
      <div className="min-w-0 [perspective:1200px]">
        <div
          className={cn(
            "grid min-w-0 w-full transition-transform duration-500 ease-[cubic-bezier(0.2,0.8,0.2,1)] will-change-transform motion-reduce:duration-0 [transform-style:preserve-3d]",
            isFlipped ? "[transform:rotateY(180deg)]" : "[transform:rotateY(0deg)]"
          )}
        >
          <div
            className={cn(
              "col-start-1 row-start-1 flex min-w-0 flex-col [backface-visibility:hidden]",
              isFlipped ? "pointer-events-none" : "pointer-events-auto"
            )}
          >
            <div className="flex items-center justify-between text-xs uppercase tracking-[0.3em] text-ink/60">
              <span>Offer</span>
              <div className="flex items-center gap-3">
                <span>{purchased} purchased</span>
                <button
                  type="button"
                  onClick={() => setIsFlipped(true)}
                  className="rounded-full border border-white/10 px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-ink/70 transition hover:border-white/40 hover:text-ink"
                >
                  Flip
                </button>
              </div>
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
            <div className="mt-auto flex items-center justify-between pt-4 text-sm">
              <div className="w-full space-y-2">
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
              </div>
            </div>
          </div>

          <div
            className={cn(
              "col-start-1 row-start-1 flex min-w-0 flex-col [backface-visibility:hidden] [transform:rotateY(180deg)]",
              isFlipped ? "pointer-events-auto" : "pointer-events-none"
            )}
          >
            <div className="flex items-center justify-between text-xs uppercase tracking-[0.3em] text-ink/60">
              <span>Offer details</span>
              <button
                type="button"
                onClick={() => setIsFlipped(false)}
                className="rounded-full border border-white/10 px-3 py-1 text-[10px] font-semibold uppercase tracking-[0.2em] text-ink/70 transition hover:border-white/40 hover:text-ink"
              >
                Back
              </button>
            </div>
            <div className="mt-4 space-y-3 text-sm text-ink/70">
              <div>
                <p className="text-xs uppercase tracking-[0.2em] text-ink/50">
                  Input guidance
                </p>
                <p className="mt-2 text-sm text-ink">{guidanceText}</p>
              </div>
              <label className="block">
                <span className="text-xs uppercase tracking-[0.2em] text-ink/50">
                  Input for this offer
                </span>
                <textarea
                  value={inputText}
                  onChange={(event) => setInputText(event.target.value)}
                  placeholder={guidanceText}
                  rows={3}
                  className="mt-2 w-full rounded-xl border border-white/10 bg-ink/10 px-3 py-2 text-sm text-ink placeholder:text-ink/40"
                />
              </label>
              <div className="flex items-center justify-between gap-3">
                <button
                  type="button"
                  onClick={handleCopyPrompt}
                  className="rounded-full border border-white/10 px-4 py-2 text-xs font-semibold uppercase tracking-[0.2em] text-ink/80 transition hover:border-white/40 hover:text-ink"
                >
                  Copy buy prompt
                </button>
                <span className="text-xs text-ink/50">
                  {copyStatus === "copied"
                    ? "Copied!"
                    : copyStatus === "error"
                      ? "Clipboard unavailable"
                      : " "}
                </span>
              </div>
            </div>
            <div className="mt-4 rounded-xl border border-white/10 bg-ink/10 p-3">
              <p className="text-xs uppercase tracking-[0.2em] text-ink/50">
                JSON representation
              </p>
              <pre className="mt-2 max-h-40 overflow-y-auto overflow-x-hidden whitespace-pre-wrap break-words text-xs text-ink/80">
                {offerJson}
              </pre>
            </div>
          </div>
        </div>
      </div>
    </TiltCard>
  );
}
