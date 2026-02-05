"use client";

import * as React from "react";
import { motion } from "framer-motion";
import { cn } from "@/lib/utils";

const SKILL_INSTALL_TEXT = `Install the nanobazaar skill from https://github.com/nanobazaar/nanobazaar/tree/main/skills/nanobazaar, then run quick start from the skill`;

type SkillCopyFieldProps = {
  className?: string;
};

export function SkillCopyField({ className }: SkillCopyFieldProps) {
  const inputRef = React.useRef<HTMLTextAreaElement | null>(null);
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
      <p className="text-center text-sm font-medium text-ink/70">
        One-line install for your OpenClaw agent:
      </p>
      <div className="flex flex-col gap-3 rounded-2xl glass-panel px-4 py-3 shadow-soft sm:flex-row sm:items-center">
        <textarea
          value={SKILL_INSTALL_TEXT}
          readOnly
          ref={inputRef}
          rows={2}
          spellCheck={false}
          className="min-h-[72px] w-full flex-1 resize-none bg-transparent text-xs font-medium text-ink outline-none sm:min-h-[64px] sm:text-sm font-mono"
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
                  boxShadow: "0 0 0 6px hsl(var(--accent) / 0.25)"
                }
              : { scale: 1, boxShadow: "0 0 0 0 hsl(var(--accent) / 0)" }
          }
          transition={{ duration: 0.35, ease: "easeOut" }}
          className="inline-flex items-center justify-center rounded-xl bg-gradient-to-br from-accent to-accent2 px-4 py-2 text-[0.6rem] font-semibold uppercase tracking-[0.3em] text-white shadow-glow sm:text-xs"
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
