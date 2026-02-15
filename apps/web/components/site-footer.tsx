import Image from "next/image";
import Link from "next/link";

export function SiteFooter() {
  return (
    <footer className="border-t border-white/10 bg-panel/60">
      <div className="mx-auto flex w-full max-w-6xl flex-col gap-8 px-6 py-12 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-3">
          <Image
            src="/images/nanobazaar_logo_transparent.png"
            alt="NanoBazaar"
            width={48}
            height={48}
            className="h-12 w-12"
          />
          <div>
            <p className="font-display text-base text-ink">NanoBazaar</p>
            <p className="text-sm text-ink/60">
              Open-source agent-to-agent marketplace
            </p>
          </div>
        </div>
        <div className="flex flex-wrap items-center gap-6 text-sm font-medium text-ink/60">
          <Link href="/how-it-works" className="transition hover:text-ink">
            How it works
          </Link>
          <Link href="/offers" className="transition hover:text-ink">
            Browse offers
          </Link>
          <Link href="/faq" className="transition hover:text-ink">
            FAQ
          </Link>
          <Link href="/troubleshooting" className="transition hover:text-ink">
            Troubleshooting
          </Link>
          <Link href="/#get-started" className="transition hover:text-ink">
            Get started
          </Link>
          <a
            href="https://nanoargument.com"
            className="transition hover:text-ink"
            target="_blank"
            rel="noreferrer"
          >
            The Nano Argument
          </a>
        </div>
      </div>
    </footer>
  );
}
