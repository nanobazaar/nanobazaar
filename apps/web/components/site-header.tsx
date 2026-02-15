"use client";

import { useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";

const GITHUB_URL = "https://github.com/nanobazaar/nanobazaar";
const TWITTER_URL = "https://x.com/TheNanoBazaar";

export function SiteHeader() {
  const [mobileOpen, setMobileOpen] = useState(false);

  const navItems = [
    { href: "/offers", label: "Browse offers" },
    { href: "/how-it-works", label: "How it works" },
    { href: "/faq", label: "FAQ" },
    { href: "/troubleshooting", label: "Troubleshooting" }
  ];

  return (
    <header className="sticky top-0 z-50 border-b border-white/10 bg-bg/70 backdrop-blur">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-4">
        <Link href="/" className="flex items-center gap-3">
          <Image
            src="/images/nanobazaar_logo_transparent.png"
            alt="NanoBazaar"
            width={44}
            height={44}
            className="h-11 w-11"
          />
          <span className="font-display text-lg tracking-tight text-ink">
            NanoBazaar
          </span>
        </Link>
        <nav className="hidden items-center gap-6 text-sm font-medium text-ink/70 md:flex">
          {navItems.map((item) => (
            <Link key={item.href} href={item.href} className="transition hover:text-ink">
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="hidden items-center gap-3 md:flex">
          <a
            href={GITHUB_URL}
            className="inline-flex items-center justify-center rounded-full border border-white/10 bg-white/5 p-2 text-ink/70 transition hover:border-white/20 hover:text-ink"
            target="_blank"
            rel="noreferrer"
            aria-label="GitHub"
          >
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              className="h-4 w-4 fill-current"
            >
              <path d="M12 .296c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.387.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.387-1.333-1.757-1.333-1.757-1.09-.745.083-.73.083-.73 1.205.085 1.84 1.236 1.84 1.236 1.07 1.834 2.809 1.304 3.495.997.108-.775.418-1.305.762-1.605-2.665-.305-5.467-1.332-5.467-5.93 0-1.31.469-2.381 1.236-3.221-.124-.304-.536-1.527.117-3.176 0 0 1.008-.322 3.301 1.23.957-.266 1.984-.399 3.006-.404 1.022.005 2.049.138 3.008.404 2.291-1.552 3.297-1.23 3.297-1.23.655 1.649.243 2.872.119 3.176.77.84 1.235 1.911 1.235 3.221 0 4.61-2.807 5.624-5.479 5.921.43.372.823 1.102.823 2.222 0 1.606-.015 2.896-.015 3.286 0 .321.216.694.825.576 4.765-1.589 8.199-6.084 8.199-11.386 0-6.627-5.373-12-12-12z" />
            </svg>
          </a>
          <a
            href={TWITTER_URL}
            className="inline-flex items-center justify-center rounded-full border border-white/10 bg-white/5 p-2 text-ink/70 transition hover:border-white/20 hover:text-ink"
            target="_blank"
            rel="noreferrer"
            aria-label="X / Twitter"
          >
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              className="h-4 w-4 fill-current"
            >
              <path d="M18.901 2h3.474l-7.592 8.677L23 22h-6.828l-5.343-6.876L3.784 22H.31l8.117-9.279L0 2h6.999l4.833 6.274L18.901 2zm-1.219 18.05h1.923L6.037 3.85H4.07L17.682 20.05z" />
            </svg>
          </a>
          <Button asChild size="sm">
            <Link href="/#get-started">Get started</Link>
          </Button>
        </div>
        <div className="relative md:hidden">
          <button
            type="button"
            className="inline-flex cursor-pointer items-center justify-center rounded-lg border border-white/10 bg-white/5 p-2 text-ink/80 transition hover:border-white/20 hover:text-ink"
            aria-expanded={mobileOpen}
            aria-controls="mobile-menu"
            aria-label="Toggle navigation menu"
            onClick={() => setMobileOpen((open) => !open)}
          >
            <svg
              aria-hidden="true"
              viewBox="0 0 24 24"
              className="h-5 w-5 fill-none stroke-current stroke-2"
            >
              <path d={mobileOpen ? "M6 6l12 12M18 6L6 18" : "M3 6h18M3 12h18M3 18h18"} />
            </svg>
          </button>
          {mobileOpen ? (
            <div
              id="mobile-menu"
              className="absolute right-0 top-[calc(100%+0.75rem)] z-50 w-64 rounded-2xl border border-white/10 bg-panel-2/95 p-3 shadow-soft backdrop-blur"
            >
              <nav className="flex flex-col text-sm font-medium text-ink/80">
                {navItems.map((item) => (
                  <Link
                    key={item.href}
                    href={item.href}
                    onClick={() => setMobileOpen(false)}
                    className="rounded-lg px-3 py-2 transition hover:bg-white/5 hover:text-ink"
                  >
                    {item.label}
                  </Link>
                ))}
                <Link
                  href="/#get-started"
                  onClick={() => setMobileOpen(false)}
                  className="mt-1 rounded-lg bg-gradient-to-br from-accent to-accent2 px-3 py-2 text-center text-xs font-semibold uppercase tracking-[0.12em] text-white"
                >
                  Get started
                </Link>
                <div className="mt-2 grid grid-cols-2 gap-2">
                  <a
                    href={GITHUB_URL}
                    onClick={() => setMobileOpen(false)}
                    className="inline-flex items-center justify-center rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs font-semibold uppercase tracking-[0.12em] text-ink/80 transition hover:border-white/20 hover:text-ink"
                    target="_blank"
                    rel="noreferrer"
                  >
                    GitHub
                  </a>
                  <a
                    href={TWITTER_URL}
                    onClick={() => setMobileOpen(false)}
                    className="inline-flex items-center justify-center rounded-lg border border-white/10 bg-white/5 px-3 py-2 text-xs font-semibold uppercase tracking-[0.12em] text-ink/80 transition hover:border-white/20 hover:text-ink"
                    target="_blank"
                    rel="noreferrer"
                  >
                    X
                  </a>
                </div>
              </nav>
            </div>
          ) : null}
        </div>
      </div>
    </header>
  );
}
