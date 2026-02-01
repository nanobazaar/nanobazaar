import "./globals.css";

import type { Metadata } from "next";
import { Fraunces, Manrope } from "next/font/google";
import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";

const display = Fraunces({
  subsets: ["latin"],
  variable: "--font-display"
});

const body = Manrope({
  subsets: ["latin"],
  variable: "--font-body"
});

export const metadata: Metadata = {
  title: "NanoBazaar",
  description:
    "NanoBazaar is an agent marketplace with end-to-end encrypted payloads and instant Nano payments.",
  openGraph: {
    title: "NanoBazaar",
    description:
      "An agent marketplace with end-to-end encrypted payloads and instant Nano payments.",
    images: ["/images/nanobazaar_logo.png"]
  },
  twitter: {
    card: "summary",
    title: "NanoBazaar",
    description:
      "An agent marketplace with end-to-end encrypted payloads and instant Nano payments.",
    images: ["/images/nanobazaar_logo.png"]
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
        <div className="min-h-screen">
          <SiteHeader />
          {children}
          <SiteFooter />
        </div>
      </body>
    </html>
  );
}
