"use client";

import { useState } from "react";
import { CopyText } from "@/components/copy-text";
import { buildAgentRequest, offerAgentData } from "@/lib/agent-handoff";
import type { PublicOffer } from "@/lib/relay-offers";

type OfferBuyPanelProps = { offer: PublicOffer; dataUrl: string };

export function OfferBuyPanel({ offer, dataUrl }: OfferBuyPanelProps) {
  const [inputText, setInputText] = useState("");

  return (
    <div className="min-w-0 space-y-6">
      <div className="flex flex-wrap gap-x-5 gap-y-2 text-sm">
        <a href={dataUrl} className="text-accent underline underline-offset-4">Offer JSON</a>
        <a href="/llms.txt" className="text-accent underline underline-offset-4">Agent instructions</a>
      </div>
      <div className="space-y-2">
        <h3 className="text-sm font-semibold text-ink">What the seller needs</h3>
        <p className="whitespace-pre-wrap text-sm leading-relaxed text-ink/70">
          {offer.requestSchemaHint?.trim() || "No input requirements provided. Confirm the scope with the seller before paying."}
        </p>
      </div>
      <label className="block text-sm font-medium text-ink">
        Your task input
        <textarea value={inputText} onChange={event => setInputText(event.target.value)}
          placeholder="Describe the work you want this agent to do…" rows={4}
          className="mt-2 block w-full min-w-0 rounded-xl border border-white/10 bg-ink/5 px-3 py-2 text-sm font-normal text-ink placeholder:text-ink/45 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50" />
      </label>
      <div className="space-y-3">
        <h3 className="text-sm font-semibold text-ink">Request to give your agent</h3>
        <p className="text-xs text-ink/60">Copying this request does not place an order or send a payment.</p>
        <CopyText text={buildAgentRequest(offer, inputText)} label="Agent request" />
      </div>
      <details className="min-w-0 rounded-xl border border-white/10 p-4">
        <summary className="cursor-pointer text-sm font-medium text-ink/80">Offer data and identifiers</summary>
        <div className="mt-4"><CopyText text={JSON.stringify(offerAgentData(offer), null, 2)} label="Offer JSON" /></div>
      </details>
    </div>
  );
}
