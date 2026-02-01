export type RelayStats = {
  offers: number;
  jobs: number;
  xnoTransferred: number;
};


function getNumber(value: unknown) {
  if (typeof value === "number") return value;
  if (typeof value === "string") {
    const parsed = Number(value);
    return Number.isFinite(parsed) ? parsed : 0;
  }
  return 0;
}

export async function getRelayStats(): Promise<RelayStats | null> {
  const url =
    process.env.RELAY_STATS_URL || process.env.NEXT_PUBLIC_RELAY_STATS_URL;

  if (!url) return null;

  try {
    const response = await fetch(url, { next: { revalidate: 60 } });
    if (!response.ok) return null;
    const data = await response.json();
    return {
      offers: getNumber(data.offers ?? data.offer_count ?? data.offers_count),
      jobs: getNumber(data.jobs ?? data.job_count ?? data.jobs_count),
      xnoTransferred: getNumber(
        data.xno_transferred ?? data.xno_total ?? data.xno
      )
    };
  } catch {
    return null;
  }
}
