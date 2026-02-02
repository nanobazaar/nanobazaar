import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";

const GITHUB_URL = "https://github.com/nanobazaar/nanobazaar";
const TWITTER_URL = "https://x.com/TheNanoBazaar";

export function SiteHeader() {
  return (
    <header className="sticky top-0 z-50 border-b border-line/70 bg-bg/80 backdrop-blur">
      <div className="mx-auto flex w-full max-w-6xl items-center justify-between px-6 py-4">
        <Link href="/" className="flex items-center gap-3">
          <Image
            src="/images/nanobazaar_logo.png"
            alt="NanoBazaar"
            width={36}
            height={36}
            className="h-9 w-9"
          />
          <span className="font-display text-lg tracking-tight">NanoBazaar</span>
        </Link>
        <nav className="hidden items-center gap-6 text-sm font-medium text-muted md:flex">
          <Link href="/how-it-works" className="transition hover:text-ink">
            How it works
          </Link>
          <Link href="/offers" className="transition hover:text-ink">
            Browse offers
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
          <a
            href={TWITTER_URL}
            className="transition hover:text-ink"
            target="_blank"
            rel="noreferrer"
          >
            X / Twitter
          </a>
        </nav>
        <div className="flex items-center gap-3">
          <Button asChild variant="outline" size="sm" className="hidden md:inline-flex">
            <Link href="/how-it-works">How it works</Link>
          </Button>
          <Button asChild size="sm">
            <Link href="/#get-started">Get started</Link>
          </Button>
        </div>
      </div>
    </header>
  );
}

export { GITHUB_URL, TWITTER_URL };
