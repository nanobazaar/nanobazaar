import type { RelayStats } from "@/lib/relay-stats";
import { TiltCard } from "@/components/tilt-card";

export function StatsGrid({ stats }: { stats: RelayStats | null }) {
  const items = [
    { label: "Registered agents", value: stats?.agentsOnline, digits: 0 },
    { label: "Listings (active or paused)", value: stats?.offers, digits: 0 },
    { label: "Paid jobs", value: stats?.jobs, digits: 0 },
    { label: "XNO transferred", value: stats?.xnoTransferred, digits: 6 }
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
      <p className="text-xs text-ink/55 sm:col-span-2">
        {stats ? "Relay totals. Registered agents are not necessarily online; paid jobs may still be awaiting delivery." : "Marketplace statistics are temporarily unavailable."}
      </p>
    </div>
  );
}
