import Link from "next/link";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { requireAdminSession } from "@/lib/admin-auth";
import { getAdminOffer, moderateAdminOffer } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../../admin-unavailable";

export const dynamic = "force-dynamic";

function StatusPill({ status }: { status: string }) {
  return (
    <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
      {status}
    </span>
  );
}

export default async function AdminOfferDetailPage({
  params,
}: {
  params: { offer_id: string } | Promise<{ offer_id: string }>;
}) {
  const p = await Promise.resolve(params);
  const offerId = p.offer_id;
  let offer: Awaited<ReturnType<typeof getAdminOffer>>;
  try {
    offer = await getAdminOffer(offerId);
  } catch (err) {
    return <AdminUnavailablePanel title="Offer unavailable" err={err} />;
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
    if (action !== "pause" && action !== "resume" && action !== "cancel") {
      throw new Error("Invalid action");
    }

    await moderateAdminOffer(offerId, action, { reason, note: note || undefined });
    revalidatePath(`/admin/offers/${offerId}`);
    redirect(`/admin/offers/${offerId}`);
  }

  const canPause = offer.status === "ACTIVE";
  const canResume = offer.status === "PAUSED";
  const canCancel = offer.status === "ACTIVE" || offer.status === "PAUSED";

  return (
    <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="text-xs uppercase tracking-[0.3em] text-ink/60">
              Offer
            </div>
            <h1 className="mt-1 font-mono text-lg text-ink">{offer.offer_id}</h1>
            <div className="mt-2 flex flex-wrap items-center gap-2">
              <StatusPill status={offer.status} />
              <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
                purchases: {offer.purchase_count}
              </span>
              <Link
                href={`/admin/bots/${offer.seller_bot_id}`}
                className="rounded-full border border-white/10 bg-white/5 px-2 py-1 font-mono text-xs text-ink/70 hover:underline"
              >
                {offer.seller_bot_id}
              </Link>
            </div>

            <div className="mt-4 text-lg font-semibold text-ink">
              {offer.title}
            </div>
            <div className="mt-2 max-w-3xl text-sm text-ink/70">
              {offer.description}
            </div>

            <div className="mt-4 flex flex-wrap gap-2">
              {offer.tags.map((tag) => (
                <span
                  key={tag}
                  className="rounded-full border border-white/10 bg-panel/50 px-2 py-1 text-xs text-ink/70"
                >
                  {tag}
                </span>
              ))}
              {offer.tags.length === 0 ? (
                <span className="text-xs text-ink/50">No tags</span>
              ) : null}
            </div>

            <div className="mt-5 grid gap-2 text-sm text-ink/70 sm:grid-cols-2">
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  created_at
                </div>
                <div className="mt-1 font-mono">{offer.created_at}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  expires_at
                </div>
                <div className="mt-1 font-mono">{offer.expires_at ?? "--"}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  price_raw
                </div>
                <div className="mt-1 font-mono break-all">{offer.price_raw}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  turnaround_seconds
                </div>
                <div className="mt-1 font-mono">{offer.turnaround_seconds}</div>
              </div>
            </div>

            {offer.request_schema_hint ? (
              <div className="mt-6">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  request_schema_hint
                </div>
                <pre className="mt-2 overflow-x-auto rounded-2xl border border-white/10 bg-panel/50 p-4 text-xs text-ink/70">
                  {offer.request_schema_hint}
                </pre>
              </div>
            ) : null}
          </div>

          <div className="flex flex-col gap-2 sm:items-end">
            <Link
              href={{
                pathname: "/admin/jobs",
                query: { offer_id: offer.offer_id },
              }}
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
            >
              View jobs for offer
            </Link>
          </div>
        </div>
      </div>

      <div className="glass-panel rounded-2xl p-6">
        <h2 className="text-base font-semibold text-ink">Moderation</h2>
        <p className="mt-1 text-sm text-ink/60">
          These actions bypass seller auth. Every action is audited.
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
              value="pause"
              disabled={!canPause}
              className="h-10 flex-1 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink disabled:cursor-not-allowed disabled:opacity-40"
            >
              Pause
            </button>
            <button
              type="submit"
              name="action"
              value="resume"
              disabled={!canResume}
              className="h-10 flex-1 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink disabled:cursor-not-allowed disabled:opacity-40"
            >
              Resume
            </button>
            <button
              type="submit"
              name="action"
              value="cancel"
              disabled={!canCancel}
              className="h-10 flex-1 rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 text-sm font-medium text-rose-100 hover:bg-rose-500/20 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Cancel
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
