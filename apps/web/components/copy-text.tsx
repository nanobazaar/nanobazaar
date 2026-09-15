"use client";

import { useEffect, useRef, useState } from "react";
import { Button } from "@/components/ui/button";

export function CopyText({ text, label }: { text: string; label: string }) {
  const content = useRef<HTMLPreElement>(null);
  const [status, setStatus] = useState<"idle" | "copied" | "error">("idle");

  useEffect(() => { setStatus("idle"); }, [text]);
  useEffect(() => {
    if (status !== "copied") return;
    const timer = window.setTimeout(() => setStatus("idle"), 2000);
    return () => window.clearTimeout(timer);
  }, [status]);

  async function copy() {
    try {
      await navigator.clipboard.writeText(text);
      setStatus("copied");
    } catch {
      const selection = window.getSelection();
      if (content.current && selection) {
        const range = document.createRange();
        range.selectNodeContents(content.current);
        selection.removeAllRanges();
        selection.addRange(range);
      }
      setStatus("error");
    }
  }

  return (
    <div className="min-w-0 space-y-3">
      <pre ref={content} tabIndex={0} aria-label={label} className="max-h-80 overflow-y-auto rounded-xl border border-white/10 bg-bg/60 p-4 text-left font-mono text-xs leading-relaxed text-ink/80 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent/50">{text}</pre>
      <div className="flex flex-wrap items-center gap-3">
        <Button type="button" size="sm" variant="outline" onClick={copy}>
          {status === "copied" ? "Copied" : `Copy ${label.toLowerCase()}`}
        </Button>
        <span role="status" className="text-xs text-ink/60">
          {status === "error" ? "Copy unavailable. Select the text above and copy it manually." : status === "copied" ? "Copied to clipboard." : ""}
        </span>
      </div>
    </div>
  );
}
