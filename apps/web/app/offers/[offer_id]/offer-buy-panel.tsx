"use client";

import * as React from "react";

import type { PublicOffer } from "@/lib/relay-offers";

type OfferBuyPanelProps = {
  offer: PublicOffer;
};

export function OfferBuyPanel({ offer }: OfferBuyPanelProps) {
  const [inputText, setInputText] = React.useState("");
  const [copyStatus, setCopyStatus] = React.useState<
    "idle" | "copied" | "error"
  >("idle");

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
          ...(offer.sellerBotName
            ? { seller_bot_name: offer.sellerBotName }
            : {}),
          title: offer.title,
          description: offer.description,
          tags: offer.tags,
          price_raw: offer.priceRaw,
          turnaround_seconds: offer.turnaroundSeconds,
          purchase_count: offer.purchaseCount,
          created_at: offer.createdAt,
          ...(requestSchemaHint ? { request_schema_hint: requestSchemaHint } : {})
        },
        null,
        2
      ),
    [
      offer.createdAt,
      offer.description,
      offer.offerId,
      offer.priceRaw,
      offer.purchaseCount,
      offer.sellerBotName,
      offer.tags,
      offer.title,
      offer.turnaroundSeconds,
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
    <div className="space-y-4">
      <div className="space-y-3 text-sm text-ink/70">
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
            rows={4}
            className="mt-2 w-full rounded-xl border border-white/10 bg-ink/10 px-3 py-2 text-sm text-ink placeholder:text-ink/40"
          />
        </label>

        <div className="flex items-center justify-between gap-3">
          <button
            type="button"
            onClick={handleCopyPrompt}
            className="rounded-full border border-white/10 px-4 py-2 text-xs font-semibold uppercase tracking-[0.2em] text-ink/80 transition hover:border-white/40 hover:text-ink"
          >
            Buy via agent
          </button>
          <span className="text-xs text-ink/50">
            {copyStatus === "copied"
              ? "Prompt copied!"
              : copyStatus === "error"
                ? "Clipboard unavailable"
                : " "}
          </span>
        </div>
      </div>

      <div className="rounded-xl border border-white/10 bg-ink/10 p-3">
        <p className="text-xs uppercase tracking-[0.2em] text-ink/50">
          JSON representation
        </p>
        <pre className="mt-2 max-h-72 overflow-y-auto overflow-x-hidden whitespace-pre-wrap break-words text-xs text-ink/80">
          {offerJson}
        </pre>
      </div>
    </div>
  );
}

