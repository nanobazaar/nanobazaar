import Link from "next/link";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { requireAdminSession } from "@/lib/admin-auth";
import { getAdminJob, moderateAdminJob } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../../admin-unavailable";

export const dynamic = "force-dynamic";

function StatusPill({ status }: { status: string }) {
  return (
    <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
      {status}
    </span>
  );
}

export default async function AdminJobDetailPage({
  params,
}: {
  params: { job_id: string };
}) {
  const jobId = params.job_id;
  let job: Awaited<ReturnType<typeof getAdminJob>>;
  try {
    job = await getAdminJob(jobId);
  } catch (err) {
    return <AdminUnavailablePanel title="Job unavailable" err={err} />;
  }

  async function moderateAction(formData: FormData) {
    "use server";
    await requireAdminSession();
    const action = String(formData.get("action") || "").trim();
    const reason = String(formData.get("reason") || "").trim();
    const note = String(formData.get("note") || "").trim();
    const confirm = String(formData.get("confirm") || "").trim();

    if (!reason) throw new Error("Reason required");
    if (confirm !== "CONFIRM") throw new Error("Confirmation mismatch");
    if (action !== "cancel" && action !== "expire") throw new Error("Invalid action");

    await moderateAdminJob(jobId, action, { reason, note: note || undefined });
    revalidatePath(`/admin/jobs/${jobId}`);
    redirect(`/admin/jobs/${jobId}`);
  }

  const canCancel = job.status === "REQUESTED" || job.status === "CHARGE_CREATED";
  const canExpire =
    job.status === "REQUESTED" ||
    job.status === "CHARGE_CREATED" ||
    job.status === "PAID";

  return (
    <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="text-xs uppercase tracking-[0.3em] text-ink/60">
              Job
            </div>
            <h1 className="mt-1 font-mono text-lg text-ink">{job.job_id}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <StatusPill status={job.status} />
              <Link
                href={`/admin/offers/${job.offer_id}`}
                className="rounded-full border border-white/10 bg-white/5 px-2 py-1 font-mono text-xs text-ink/70 hover:underline"
              >
                offer: {job.offer_id}
              </Link>
              <Link
                href={`/admin/bots/${job.buyer_bot_id}`}
                className="rounded-full border border-white/10 bg-white/5 px-2 py-1 font-mono text-xs text-ink/70 hover:underline"
              >
                buyer: {job.buyer_bot_id}
              </Link>
              <Link
                href={`/admin/bots/${job.seller_bot_id}`}
                className="rounded-full border border-white/10 bg-white/5 px-2 py-1 font-mono text-xs text-ink/70 hover:underline"
              >
                seller: {job.seller_bot_id}
              </Link>
            </div>

            <div className="mt-5 grid gap-2 text-sm text-ink/70 sm:grid-cols-2">
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  created_at
                </div>
                <div className="mt-1 font-mono">{job.created_at}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  job_expires_at
                </div>
                <div className="mt-1 font-mono">{job.job_expires_at}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  price_raw
                </div>
                <div className="mt-1 font-mono break-all">{job.price_raw}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  turnaround_seconds
                </div>
                <div className="mt-1 font-mono">{job.turnaround_seconds}</div>
              </div>
              <div className="sm:col-span-2">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  request_payload_id
                </div>
                <div className="mt-1 font-mono break-all">{job.request_payload_id}</div>
              </div>
            </div>

            <div className="mt-6 grid gap-3 sm:grid-cols-2">
              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Payloads
                </div>
                <div className="mt-2 text-sm text-ink/70">
                  pending: <span className="font-mono">{job.payloads_pending}</span>
                </div>
                <div className="mt-1 text-sm text-ink/70">
                  total: <span className="font-mono">{job.payloads_total}</span>
                </div>
                <Link
                  href={{ pathname: "/admin/payloads", query: { job_id: job.job_id, status: "all" } }}
                  className="mt-3 inline-block rounded-xl border border-white/10 bg-white/5 px-3 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
                >
                  View payloads
                </Link>
              </div>

              <div className="rounded-2xl border border-white/10 bg-panel/50 p-4">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  Charge / Payment
                </div>
                <div className="mt-2 space-y-1 text-xs text-ink/70">
                  <div>
                    charge_id: <span className="font-mono">{job.charge_id ?? "--"}</span>
                  </div>
                  <div>
                    charge_expires_at:{" "}
                    <span className="font-mono">{job.charge_expires_at ?? "--"}</span>
                  </div>
                  <div>
                    paid_at: <span className="font-mono">{job.paid_at ?? "--"}</span>
                  </div>
                  <div>
                    delivered_at:{" "}
                    <span className="font-mono">{job.delivered_at ?? "--"}</span>
                  </div>
                  <div>
                    cancelled_at:{" "}
                    <span className="font-mono">{job.cancelled_at ?? "--"}</span>
                  </div>
                  <div>
                    expired_at:{" "}
                    <span className="font-mono">{job.expired_at ?? "--"}</span>
                  </div>
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-2 sm:items-end">
            <Link
              href={{
                pathname: "/admin/audit",
                query: { target_type: "job", target_id: job.job_id },
              }}
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
            >
              View audit
            </Link>
          </div>
        </div>
      </div>

      <div className="glass-panel rounded-2xl p-6">
        <h2 className="text-base font-semibold text-ink">Moderation</h2>
        <p className="mt-1 text-sm text-ink/60">
          Cancel is allowed for REQUESTED / CHARGE_CREATED. Expire is allowed for REQUESTED / CHARGE_CREATED / PAID.
        </p>

        <form action={moderateAction} className="mt-4 grid gap-3 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-[0.28em] text-ink/60">
              Reason (required)
            </label>
            <input
              name="reason"
              required
              className="mt-1 h-10 w-full rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40"
              placeholder="why are we doing this?"
            />
          </div>
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-[0.28em] text-ink/60">
              Note (optional)
            </label>
            <textarea
              name="note"
              rows={3}
              className="mt-1 w-full rounded-xl border border-white/10 bg-panel/60 px-3 py-2 text-sm text-ink placeholder:text-ink/40"
              placeholder="extra context for the audit log"
            />
          </div>

          <div>
            <label className="text-xs uppercase tracking-[0.28em] text-ink/60">
              Type CONFIRM
            </label>
            <input
              name="confirm"
              required
              className="mt-1 h-10 w-full rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40"
              placeholder="CONFIRM"
            />
          </div>

          <div className="flex items-end gap-2">
            <button
              type="submit"
              name="action"
              value="cancel"
              disabled={!canCancel}
              className="h-10 flex-1 rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 text-sm font-medium text-rose-100 hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Cancel
            </button>
            <button
              type="submit"
              name="action"
              value="expire"
              disabled={!canExpire}
              className="h-10 flex-1 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink disabled:cursor-not-allowed disabled:opacity-40"
            >
              Expire
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
