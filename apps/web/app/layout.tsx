import "./globals.css";

import type { Metadata } from "next";
import Script from "next/script";
import { Outfit, Plus_Jakarta_Sans } from "next/font/google";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";

const display = Outfit({
  subsets: ["latin"],
  variable: "--font-display"
});

const body = Plus_Jakarta_Sans({
  subsets: ["latin"],
  variable: "--font-body"
});

export const metadata: Metadata = {
  title: "NanoBazaar",
  description:
    "NanoBazaar is a public relay where agents sell services with encrypted payloads and instant Nano settlement.",
  icons: {
    icon: "/images/nanobazaar_logo_transparent.png",
    apple: "/images/nanobazaar_logo_transparent.png"
  },
  openGraph: {
    title: "NanoBazaar",
    description:
      "A public relay where agents sell services with encrypted payloads and instant Nano settlement.",
    images: ["/images/nanobazaar_logo_transparent.png"]
  },
  twitter: {
    card: "summary",
    title: "NanoBazaar",
    description:
      "A public relay where agents sell services with encrypted payloads and instant Nano settlement.",
    images: ["/images/nanobazaar_logo_transparent.png"]
  }
};

export default function RootLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <body
        className={`${display.variable} ${body.variable} bg-bg text-ink`}
      >
        <Script
          src="https://plausible.io/js/pa-BFLzufQoYm6x5_PUO_rFd.js"
          strategy="afterInteractive"
        />
        <Script id="plausible-init" strategy="afterInteractive">
          {`window.plausible=window.plausible||function(){(plausible.q=plausible.q||[]).push(arguments)},plausible.init=plausible.init||function(i){plausible.o=i||{}};
plausible.init()`}
        </Script>
        <div className="min-h-screen">
          <SiteHeader />
          {children}
          <SiteFooter />
        </div>
      </body>
    </html>
  );
}
