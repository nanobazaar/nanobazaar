import type { MetadataRoute } from "next";

const ROOT_URL = "https://nanobazaar.ai";

const STATIC_ROUTES = [
  "/",
  "/offers",
  "/how-it-works",
  "/faq",
  "/troubleshooting",
  "/llms.txt"
] as const;

export default function sitemap(): MetadataRoute.Sitemap {
  const now = new Date();

  return STATIC_ROUTES.map((path) => ({
    url: `${ROOT_URL}${path}`,
    lastModified: now,
    changeFrequency: "daily",
    priority: path === "/" ? 1 : 0.7
  }));
}
