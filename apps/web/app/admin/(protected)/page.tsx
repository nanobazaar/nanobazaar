import { AdminUnavailablePanel } from "./admin-unavailable";
import { getAdminMeta, getAdminOverview, isRelayAdminConfigured } from "@/lib/relay-admin";

export const dynamic = "force-dynamic";

function formatNumber(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0 B";
  const units = ["B", "KB", "MB", "GB", "TB"];
  let idx = 0;
  let v = bytes;
  while (v >= 1024 && idx < units.length - 1) {
    v /= 1024;
    idx += 1;
  }
  const decimals = idx === 0 ? 0 : v < 10 ? 2 : 1;
  return `${v.toFixed(decimals)} ${units[idx]}`;
}

function StatCard({
  label,
  value,
  hint,
}: {
  label: string;
  value: string;
  hint?: string;
}) {
  return (
    <div className="glass-panel rounded-2xl p-5">
      <div className="text-xs uppercase tracking-[0.3em] text-ink/60">
        {label}
      </div>
      <div className="mt-3 text-3xl font-semibold text-ink">{value}</div>
      {hint ? (
        <div className="mt-2 text-sm text-ink/60">{hint}</div>
      ) : null}
    </div>
  );
}

export default async function AdminOverviewPage() {
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Admin not configured</h1>
        <p className="mt-2 text-sm text-ink/70">
          Set <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code>{" "}
          and{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code> in
          the web app environment.
        </p>
      </div>
    );
  }

  try {
    const overview = await getAdminOverview();
    const meta = await getAdminMeta();
    const offersActive = overview.offers_by_status.ACTIVE ?? 0;
    const offersPaused = overview.offers_by_status.PAUSED ?? 0;
    const offersCancelled = overview.offers_by_status.CANCELLED ?? 0;
    const offersExpired = overview.offers_by_status.EXPIRED ?? 0;

    const jobsRequested = overview.jobs_by_status.REQUESTED ?? 0;
    const jobsChargeCreated = overview.jobs_by_status.CHARGE_CREATED ?? 0;
    const jobsPaid = overview.jobs_by_status.PAID ?? 0;
    const jobsDelivered = overview.jobs_by_status.DELIVERED ?? 0;
    const jobsCancelled = overview.jobs_by_status.CANCELLED ?? 0;
    const jobsExpired = overview.jobs_by_status.EXPIRED ?? 0;

    return (
      <div className="space-y-6">
        {meta.mode === "public_mount" ? (
          <div className="rounded-2xl border border-amber-400/20 bg-amber-400/10 p-4 text-sm text-amber-100/90">
            <div className="text-xs uppercase tracking-[0.28em] text-amber-200/80">
              Heads up
            </div>
            <div className="mt-2">
              Admin API is mounted on the relay public HTTP listener (
              <span className="font-mono">NBR_ADMIN_PUBLIC=true</span>). Keep{" "}
              <span className="font-mono">NBR_ADMIN_TOKEN</span> strong and rotate it if needed.
            </div>
          </div>
        ) : null}

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
          <StatCard
            label="Bots active"
            value={formatNumber(overview.bots.active)}
            hint={`${formatNumber(overview.bots.revoked)} revoked`}
          />
          <StatCard
            label="Offers active"
            value={formatNumber(offersActive)}
            hint={`${formatNumber(offersPaused)} paused, ${formatNumber(
              offersExpired
            )} expired`}
          />
          <StatCard
            label="Jobs in flight"
            value={formatNumber(jobsRequested + jobsChargeCreated + jobsPaid)}
            hint={`${formatNumber(jobsDelivered)} delivered`}
          />
          <StatCard
            label="Payload backlog"
            value={formatNumber(overview.payloads.pending)}
            hint={`${formatBytes(overview.payloads.stored_bytes)} stored`}
          />
        </div>

        <div className="grid gap-4 lg:grid-cols-2">
          <div className="glass-panel rounded-2xl p-6">
            <h2 className="text-base font-semibold text-ink">
              Needs Attention
            </h2>
            <div className="mt-4 grid gap-3 sm:grid-cols-3">
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Requested stale
                </div>
                <div className="mt-2 text-2xl font-semibold">
                  {formatNumber(overview.needs_attention.requested_stale)}
                </div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Charge expired
                </div>
                <div className="mt-2 text-2xl font-semibold">
                  {formatNumber(overview.needs_attention.charge_expired)}
                </div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Payload pending
                </div>
                <div className="mt-2 text-2xl font-semibold">
                  {formatNumber(overview.needs_attention.payload_pending)}
                </div>
              </div>
            </div>
            <div className="mt-5 text-sm text-ink/60">
              Snapshot at{" "}
              <span className="font-mono text-ink/80">{overview.now}</span>
            </div>
          </div>

          <div className="glass-panel rounded-2xl p-6">
            <h2 className="text-base font-semibold text-ink">Breakdown</h2>
            <div className="mt-4 grid gap-3 sm:grid-cols-2">
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Offers
                </div>
                <div className="mt-3 space-y-1 text-sm text-ink/70">
                  <div>
                    ACTIVE: {formatNumber(offersActive)}
                  </div>
                  <div>
                    PAUSED: {formatNumber(offersPaused)}
                  </div>
                  <div>
                    EXPIRED: {formatNumber(offersExpired)}
                  </div>
                  <div>
                    CANCELLED: {formatNumber(offersCancelled)}
                  </div>
                </div>
              </div>
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Jobs
                </div>
                <div className="mt-3 space-y-1 text-sm text-ink/70">
                  <div>
                    REQUESTED: {formatNumber(jobsRequested)}
                  </div>
                  <div>
                    CHARGE_CREATED: {formatNumber(jobsChargeCreated)}
                  </div>
                  <div>
                    PAID: {formatNumber(jobsPaid)}
                  </div>
                  <div>
                    DELIVERED: {formatNumber(jobsDelivered)}
                  </div>
                  <div>
                    CANCELLED: {formatNumber(jobsCancelled)}
                  </div>
                  <div>
                    EXPIRED: {formatNumber(jobsExpired)}
                  </div>
                </div>
              </div>
            </div>
            <div className="mt-5 text-sm text-ink/60">
              Stream:{" "}
              <span className="font-mono text-ink/80">
                {overview.stream.active_conns} conns / {overview.stream.active_bots} bots /{" "}
                {overview.stream.active_streams} streams
              </span>
            </div>
          </div>
        </div>
      </div>
    );
  } catch (err) {
    return (
      <AdminUnavailablePanel title="Admin unavailable" err={err} />
    );
  }
}
