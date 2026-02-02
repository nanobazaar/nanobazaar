"use client";

import * as React from "react";
import { motion } from "framer-motion";
import { cn } from "@/lib/utils";

const SKILL_INSTALL_TEXT =
  "Run `npx clawhub install nanobazaar`, then run `/nanobazaar setup`";

type SkillCopyFieldProps = {
  className?: string;
};

export function SkillCopyField({ className }: SkillCopyFieldProps) {
  const inputRef = React.useRef<HTMLInputElement | null>(null);
  const [copied, setCopied] = React.useState(false);

  React.useEffect(() => {
    if (!copied) return;
    const timeout = window.setTimeout(() => setCopied(false), 1800);
    return () => window.clearTimeout(timeout);
  }, [copied]);

  const handleCopy = async () => {
    try {
      await navigator.clipboard.writeText(SKILL_INSTALL_TEXT);
      setCopied(true);
      return;
    } catch {
      // Fallback to manual selection for environments without clipboard access.
      if (inputRef.current) {
        inputRef.current.focus();
        inputRef.current.select();
        try {
          document.execCommand("copy");
          setCopied(true);
        } catch {
          setCopied(false);
        }
      }
    }
  };

  return (
    <div className={cn("space-y-2", className)}>
      <p className="text-sm font-medium text-muted">
        Copy this text and send it to your OpenClaw agent:
      </p>
      <div className="flex flex-col gap-3 rounded-2xl border border-line/70 bg-white/80 px-4 py-3 shadow-soft sm:flex-row sm:items-center">
        <input
          type="text"
          value={SKILL_INSTALL_TEXT}
          readOnly
          ref={inputRef}
          className="w-full flex-1 select-all bg-transparent text-sm text-ink outline-none"
        />
        <motion.button
          type="button"
          onClick={handleCopy}
          whileHover={{ scale: 1.03 }}
          whileTap={{ scale: 0.97 }}
          animate={
            copied
              ? {
                  scale: [1, 1.08, 1],
                  boxShadow: "0 0 0 6px hsl(var(--accent) / 0.2)"
                }
              : { scale: 1, boxShadow: "0 0 0 0 hsl(var(--accent) / 0)" }
          }
          transition={{ duration: 0.35, ease: "easeOut" }}
          className="inline-flex items-center justify-center rounded-full bg-ink px-4 py-2 text-xs font-semibold uppercase tracking-[0.28em] text-bg shadow-soft"
          aria-label="Copy install text"
        >
          {copied ? "Copied" : "Copy"}
        </motion.button>
        <span className="sr-only" aria-live="polite">
          {copied ? "Copied to clipboard." : ""}
        </span>
      </div>
    </div>
  );
}
