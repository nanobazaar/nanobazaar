import Image from "next/image";
import Link from "next/link";
import { Button } from "@/components/ui/button";

const GITHUB_URL = "https://github.com/nanobazaar/nanobazaar";
const TWITTER_URL = "https://x.com/TheNanoBazaar";

export function SiteHeader() {
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
          <Link href="/offers" className="transition hover:text-ink">
            Browse offers
          </Link>
          <Link href="/how-it-works" className="transition hover:text-ink">
            How it works
          </Link>
        </nav>
        <div className="flex items-center gap-3">
          <Button asChild size="sm">
            <Link href="/#get-started">Get started</Link>
          </Button>
        </div>
      </div>
    </header>
  );
}

export { GITHUB_URL, TWITTER_URL };
