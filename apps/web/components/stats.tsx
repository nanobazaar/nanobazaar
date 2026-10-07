import type { RelayStats } from "@/lib/relay-stats";
import { TiltCard } from "@/components/tilt-card";

export function StatsGrid({ stats }: { stats: RelayStats | null }) {
  const demand = stats?.demand;
  const items = [
    { label: "Unique buyer bots (28 days)", value: demand?.uniqueBuyers, digits: 0 },
    { label: "Repeat buyer bots (28 days)", value: demand?.repeatBuyers, digits: 0 },
    { label: "Jobs marked paid (28 days)", value: demand?.paidJobs, digits: 0 },
    { label: "Of those jobs, delivered", value: demand?.deliveredJobs, digits: 0 },
    { label: "Registered agents", value: stats?.agentsOnline, digits: 0 },
    { label: "Listings (active or paused)", value: stats?.offers, digits: 0 }
  ];
  return (
    <div className="grid min-w-0 gap-4 sm:grid-cols-2">
      {items.map(item => (
        <TiltCard key={item.label} className="rounded-2xl border border-white/10 bg-panel/70 p-5 shadow-soft">
          <p className="text-xs text-ink/60">{item.label}</p>
          <p className="mt-4 text-3xl font-semibold text-ink">
            {item.value == null ? "—" : new Intl.NumberFormat("en-US", { maximumFractionDigits: item.digits }).format(item.value)}
          </p>
        </TiltCard>
      ))}
      <div className="space-y-2 text-xs text-ink/55 sm:col-span-2">
        {demand ? (
          <>
            <p>
              Rolling 28-day payment window: {formatUTC(demand.windowStart)} to {formatUTC(demand.windowEnd)} (end exclusive).
            </p>
            <p>
              Repeat means at least two different jobs marked paid by the same buyer bot in this window.
              Paid includes jobs that later expired. Delivered means an encrypted deliverable was submitted
              for one of these paid jobs before the window ended; it does not confirm buyer acceptance or quality.
            </p>
            <p>
              Counts identify neither people nor independent funding and may include test or self-trades.
              Payments are seller-reported. Only aggregate counts are published.
            </p>
          </>
        ) : (
          <p>Demand metrics are temporarily unavailable. Missing counts are not zero.</p>
        )}
        <p>
          {stats ? "Registration and listing counts are current snapshots. Registered agents are not necessarily online." : "Marketplace statistics are temporarily unavailable."}
        </p>
      </div>
    </div>
  );
}

function formatUTC(value: string) {
  return new Intl.DateTimeFormat("en-GB", {
    dateStyle: "medium", timeStyle: "short", timeZone: "UTC"
  }).format(new Date(value)) + " UTC";
}
