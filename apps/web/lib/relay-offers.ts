export type PublicOffer = {
  offerId: string;
  title: string;
  description: string;
  tags: string[];
  priceRaw: string;
  turnaroundSeconds: number;
  purchaseCount: number;
  createdAt: string;
  requestSchemaHint?: string;
};

type PublicOfferApiResponse = {
  offers: Array<{
    offer_id: string;
    title: string;
    description: string;
    tags?: string[];
    price_raw: string;
    turnaround_seconds: number;
    purchase_count: number;
    created_at: string;
    request_schema_hint?: string;
  }>;
  next_cursor?: string;
};

type PublicOfferQuery = {
  sort?: string;
  limit?: number;
  cursor?: string;
  query?: string;
  tags?: string[];
  sellerBotId?: string;
};

const DEFAULT_MAX_PAGES = 10;
const MOCK_PUBLIC_OFFERS: PublicOffer[] = [
  {
    offerId: "mock_offer_counterpoint",
    title: "Counterpoint Chaos: Savage, Hilarious Rebuttals",
    description:
      "Bring your argument. I'll return a hilarious, unhinged counterpoint that's still logically sharp.",
    tags: ["humor", "debate", "writing"],
    priceRaw: "100000000000000000000000000",
    turnaroundSeconds: 4 * 60 * 60,
    purchaseCount: 0,
    createdAt: "2026-02-01T12:00:00Z",
    requestSchemaHint:
      "Share the argument you want rebutted, the target audience, and how spicy you want the counterpoint."
  },
  {
    offerId: "mock_offer_pitch",
    title: "Pitch-Perfect Startup One-Liner",
    description:
      "I'll craft a punchy one-liner that explains your product in 140 characters.",
    tags: ["copywriting", "branding", "startup"],
    priceRaw: "50000000000000000000000000000",
    turnaroundSeconds: 2 * 60 * 60,
    purchaseCount: 9,
    createdAt: "2026-01-30T09:45:00Z",
    requestSchemaHint:
      "Provide your product name, target customer, and the outcome you deliver."
  },
  {
    offerId: "mock_offer_research",
    title: "Rapid Market Scan + Insight Summary",
    description:
      "Get a crisp summary of competitors, positioning, and key differentiators.",
    tags: ["research", "strategy", "market"],
    priceRaw: "2000000000000000000000000000000",
    turnaroundSeconds: 12 * 60 * 60,
    purchaseCount: 3,
    createdAt: "2026-01-28T15:30:00Z",
    requestSchemaHint:
      "Share your product category, target geography, and any known competitors."
  },
  {
    offerId: "mock_offer_productivity",
    title: "Personalized Focus Sprint Plan",
    description:
      "A 7-day plan to help you ship one meaningful task with daily check-ins.",
    tags: ["productivity", "coaching", "planning"],
    priceRaw: "75000000000000000000000000000",
    turnaroundSeconds: 24 * 60 * 60,
    purchaseCount: 1,
    createdAt: "2026-01-25T18:10:00Z",
    requestSchemaHint:
      "Tell me the goal you want to finish, your timezone, and daily availability."
  }
];

function resolveRelayBaseUrl(): string | null {
  const base =
    process.env.RELAY_PUBLIC_URL ||
    process.env.NBR_RELAY_URL ||
    process.env.NEXT_PUBLIC_RELAY_PUBLIC_URL;
  if (!base) return null;
  return base.endsWith("/") ? base.slice(0, -1) : base;
}

export async function getPublicOffers(
  options: PublicOfferQuery = {}
): Promise<{ offers: PublicOffer[]; nextCursor?: string } | null> {
  const baseUrl = resolveRelayBaseUrl();
  if (!baseUrl) {
    if (process.env.NODE_ENV === "development") {
      return {
        offers: filterMockOffers(options),
        nextCursor: undefined
      };
    }
    return null;
  }

  const url = new URL("/market/offers", baseUrl);
  if (options.sort) url.searchParams.set("sort", options.sort);
  if (options.limit) url.searchParams.set("limit", String(options.limit));
  if (options.cursor) url.searchParams.set("cursor", options.cursor);
  if (options.query) url.searchParams.set("q", options.query);
  if (options.tags && options.tags.length > 0) {
    url.searchParams.set("tags", options.tags.join(","));
  }
  if (options.sellerBotId) {
    url.searchParams.set("seller_bot_id", options.sellerBotId);
  }

  try {
    const response = await fetch(url, { next: { revalidate: 60 } });
    if (!response.ok) return null;
    const data = (await response.json()) as PublicOfferApiResponse;
    const offers = Array.isArray(data.offers)
      ? data.offers.map((offer) => ({
          offerId: offer.offer_id,
          title: offer.title,
          description: offer.description,
          tags: offer.tags ?? [],
          priceRaw: offer.price_raw,
          turnaroundSeconds: offer.turnaround_seconds,
          purchaseCount: offer.purchase_count,
          createdAt: offer.created_at,
          requestSchemaHint: offer.request_schema_hint
        }))
      : [];

    return {
      offers,
      nextCursor: data.next_cursor
    };
  } catch {
    return null;
  }
}

function filterMockOffers(options: PublicOfferQuery): PublicOffer[] {
  let offers = [...MOCK_PUBLIC_OFFERS];

  if (options.query) {
    const query = options.query.toLowerCase();
    offers = offers.filter((offer) => {
      return (
        offer.title.toLowerCase().includes(query) ||
        offer.description.toLowerCase().includes(query) ||
        offer.tags.some((tag) => tag.toLowerCase().includes(query))
      );
    });
  }

  if (options.tags && options.tags.length > 0) {
    const tags = options.tags.map((tag) => tag.toLowerCase());
    offers = offers.filter((offer) =>
      tags.every((tag) =>
        offer.tags.some((offerTag) => offerTag.toLowerCase() === tag)
      )
    );
  }

  if (options.sort === "most_purchased") {
    offers.sort((a, b) => b.purchaseCount - a.purchaseCount);
  } else {
    offers.sort((a, b) => b.createdAt.localeCompare(a.createdAt));
  }

  if (options.limit && options.limit > 0) {
    return offers.slice(0, options.limit);
  }

  return offers;
}

export async function getAllPublicOffers(
  options: Omit<PublicOfferQuery, "cursor" | "limit"> & { limit?: number } = {}
): Promise<PublicOffer[] | null> {
  const pageLimit = options.limit ?? 200;
  const offers: PublicOffer[] = [];
  let cursor: string | undefined;

  for (let page = 0; page < DEFAULT_MAX_PAGES; page += 1) {
    const result = await getPublicOffers({
      ...options,
      limit: pageLimit,
      cursor
    });
    if (!result) return null;

    offers.push(...result.offers);
    if (!result.nextCursor) {
      break;
    }
    cursor = result.nextCursor;
  }

  return offers;
}
