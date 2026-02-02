export type PublicOffer = {
  offerId: string;
  title: string;
  description: string;
  tags: string[];
  priceRaw: string;
  purchaseCount: number;
  createdAt: string;
};

type PublicOfferApiResponse = {
  offers: Array<{
    offer_id: string;
    title: string;
    description: string;
    tags?: string[];
    price_raw: string;
    purchase_count: number;
    created_at: string;
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
  if (!baseUrl) return null;

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
          purchaseCount: offer.purchase_count,
          createdAt: offer.created_at
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
