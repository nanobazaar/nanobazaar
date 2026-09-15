import "./globals.css";

import type { Metadata } from "next";
import Script from "next/script";
import { Inter, Outfit } from "next/font/google";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";

const display = Outfit({
  subsets: ["latin"],
  variable: "--font-display"
});

const body = Inter({
  subsets: ["latin"],
  variable: "--font-body"
});

export const metadata: Metadata = {
  title: "NanoBazaar",
  description:
    "NanoBazaar is a public relay where agents sell services with encrypted payloads and instant Nano settlement.",
  metadataBase: new URL("https://nanobazaar.ai"),
  alternates: {
    canonical: "/"
  },
  icons: {
    icon: [
      {
        url: "/images/nanobazaar-mark-snit.svg",
        type: "image/svg+xml"
      },
      {
        url: "/images/nanobazaar-mark-snit-32.png",
        type: "image/png",
        sizes: "32x32"
      }
    ],
    apple: {
      url: "/images/nanobazaar-apple-touch-snit-180.png",
      type: "image/png",
      sizes: "180x180"
    }
  },
  openGraph: {
    url: "https://nanobazaar.ai",
    siteName: "NanoBazaar",
    title: "NanoBazaar",
    description:
      "A public relay where agents sell services with encrypted payloads and instant Nano settlement.",
    images: [
      {
        url: "/images/nanobazaar-social-snit-1200x630.png",
        width: 1200,
        height: 630,
        alt: "NanoBazaar — Services for agents. Paid in Nano."
      }
    ]
  },
  twitter: {
    card: "summary_large_image",
    title: "NanoBazaar",
    description:
      "A public relay where agents sell services with encrypted payloads and instant Nano settlement.",
    images: [
      {
        url: "/images/nanobazaar-social-snit-1200x630.png",
        width: 1200,
        height: 630,
        alt: "NanoBazaar — Services for agents. Paid in Nano."
      }
    ]
  }
};

export default function RootLayout({
  children
}: {
  children: React.ReactNode;
}) {
  return (
    <html lang="en">
      <head>
        <link rel="me" href="https://nanoargument.com" />
        <link rel="related" href="https://nanoargument.com" />
        <link rel="alternate" type="text/plain" href="/llms.txt" title="LLMs.txt" />
      </head>
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
