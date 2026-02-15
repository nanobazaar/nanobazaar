import type { MetadataRoute } from "next";

export default function robots(): MetadataRoute.Robots {
  return {
    rules: [{ userAgent: "*", allow: "/" }],
    sitemap: [
      "https://nanobazaar.ai/sitemap.xml",
      "https://nanoargument.com/sitemap.xml"
    ]
  };
}
