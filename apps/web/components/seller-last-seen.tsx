"use client";

import { useEffect, useState } from "react";

import { cn } from "@/lib/utils";

type SellerLastSeenProps = {
  at?: string | null;
  explain?: boolean;
  className?: string;
};

function relativeContact(at: number, now: number): string {
  const seconds = Math.max(0, Math.floor((now - at) / 1000));
  if (seconds < 60) return "just now";
  const format = new Intl.RelativeTimeFormat("en", { numeric: "always" });
  if (seconds < 3600) return format.format(-Math.floor(seconds / 60), "minute");
  if (seconds < 86400) return format.format(-Math.floor(seconds / 3600), "hour");
  return format.format(-Math.floor(seconds / 86400), "day");
}

export function SellerLastSeen({ at, explain = false, className }: SellerLastSeenProps) {
  const [now, setNow] = useState<number | null>(null);
  const timestamp = at ? Date.parse(at) : NaN;
  const known = Number.isFinite(timestamp);
  const iso = known ? new Date(timestamp).toISOString() : null;

  useEffect(() => {
    if (!known) return;
    setNow(Date.now());
    const timer = window.setInterval(() => setNow(Date.now()), 60_000);
    return () => window.clearInterval(timer);
  }, [known]);

  return (
    <div className={cn("text-xs leading-relaxed text-ink/60", className)}>
      <p>
        Last seen{known ? (now === null ? "" : ` ${relativeContact(timestamp, now)}`) : " unknown"}
      </p>
      {iso ? (
        <time dateTime={iso} className="block text-[11px] text-ink/50">
          {iso.replace("T", " ").replace(/\.\d{3}Z$/, " UTC")}
        </time>
      ) : null}
      {explain ? (
        <p className="mt-1 max-w-prose">
          The last contact from this seller’s bot. It does not guarantee an immediate response.
        </p>
      ) : null}
    </div>
  );
}
