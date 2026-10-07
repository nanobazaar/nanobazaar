export type RelayStats = {
  agentsOnline: number;
  offers: number;
  jobs: number;
  xnoTransferred: number;
  demand: DemandStats | null;
};

export type DemandStats = {
  windowDays: number;
  windowStart: string;
  windowEnd: string;
  paidJobs: number;
  deliveredJobs: number;
  uniqueBuyers: number;
  repeatBuyers: number;
};

// Missing/invalid metrics from an older relay are unknown, never a zero count.
export function parseDemandStats(value: unknown): DemandStats | null {
  if (!value || typeof value !== "object") return null;
  const data = value as Record<string, unknown>;
  const { window_days, window_start, window_end, paid_jobs, delivered_jobs, unique_buyers, repeat_buyers } = data;
  if (
    window_days !== 28 ||
    typeof window_start !== "string" || typeof window_end !== "string" ||
    !window_start.endsWith("Z") || !window_end.endsWith("Z") ||
    Date.parse(window_end) - Date.parse(window_start) !== window_days * 86400000 ||
    typeof paid_jobs !== "number" || !Number.isSafeInteger(paid_jobs) || paid_jobs < 0 ||
    typeof delivered_jobs !== "number" || !Number.isSafeInteger(delivered_jobs) || delivered_jobs < 0 ||
    typeof unique_buyers !== "number" || !Number.isSafeInteger(unique_buyers) || unique_buyers < 0 ||
    typeof repeat_buyers !== "number" || !Number.isSafeInteger(repeat_buyers) || repeat_buyers < 0 ||
    delivered_jobs > paid_jobs || unique_buyers > paid_jobs ||
    repeat_buyers > unique_buyers || unique_buyers + repeat_buyers > paid_jobs ||
    (paid_jobs > 0 && unique_buyers === 0)
  ) return null;
  return {
    windowDays: window_days, windowStart: window_start, windowEnd: window_end,
    paidJobs: paid_jobs, deliveredJobs: delivered_jobs,
    uniqueBuyers: unique_buyers, repeatBuyers: repeat_buyers
  };
}

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
      agentsOnline: getNumber(
        data.agents_online ??
          data.agents ??
          data.bots ??
          data.bot_count ??
          data.agents_count
      ),
      offers: getNumber(data.offers ?? data.offer_count ?? data.offers_count),
      jobs: getNumber(data.jobs ?? data.job_count ?? data.jobs_count),
      xnoTransferred: getNumber(
        data.xno_transferred ?? data.xno_total ?? data.xno
      ),
      demand: parseDemandStats(data.demand)
    };
  } catch {
    return null;
  }
}
