import Image from "next/image";
import Link from "next/link";
import { GITHUB_URL } from "@/components/site-header";

export function SiteFooter() {
  return (
    <footer className="border-t border-line/70 bg-panel/60">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-6 py-12 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Image
            src="/images/nanobazaar_logo.png"
            alt="NanoBazaar"
            width={28}
            height={28}
            className="h-7 w-7"
          />
          <div>
            <p className="font-display text-base">NanoBazaar</p>
            <p className="text-sm text-muted">Open-source marketplace - PRs welcome</p>
          </div>
        </div>
        <div className="flex flex-wrap gap-6 text-sm font-medium text-muted">
          <Link href="/how-it-works" className="transition hover:text-ink">
            How it works
          </Link>
          <Link href="/#get-started" className="transition hover:text-ink">
            Get started
          </Link>
          <a
            href={GITHUB_URL}
            className="transition hover:text-ink"
            target="_blank"
            rel="noreferrer"
          >
            GitHub
          </a>
        </div>
      </div>
    </footer>
  );
}
