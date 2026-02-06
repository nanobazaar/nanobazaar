import Link from "next/link";
import { revalidatePath } from "next/cache";
import { redirect } from "next/navigation";

import { requireAdminSession } from "@/lib/admin-auth";
import { getAdminBot, revokeAdminBot } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../../admin-unavailable";

export const dynamic = "force-dynamic";

export default async function AdminBotDetailPage({
  params,
}: {
  params: { bot_id: string };
}) {
  const botId = params.bot_id;
  let data: Awaited<ReturnType<typeof getAdminBot>>;
  try {
    data = await getAdminBot(botId);
  } catch (err) {
    return <AdminUnavailablePanel title="Bot unavailable" err={err} />;
  }

  async function revokeAction(formData: FormData) {
    "use server";
    await requireAdminSession();
    const reason = String(formData.get("reason") || "").trim();
    const note = String(formData.get("note") || "").trim();
    const confirm = String(formData.get("confirm") || "").trim();
    if (!reason) throw new Error("Reason required");
    if (confirm !== "REVOKE") throw new Error("Confirmation mismatch");

    await revokeAdminBot(botId, { reason, note: note || undefined });
    revalidatePath(`/admin/bots/${botId}`);
    redirect(`/admin/bots/${botId}`);
  }

  const bot = data.bot;

  return (
    <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
          <div>
            <div className="text-xs uppercase tracking-[0.3em] text-ink/60">
              Bot
            </div>
            <h1 className="mt-1 font-mono text-lg text-ink">{bot.bot_id}</h1>
            <div className="mt-2 flex flex-wrap gap-2 text-xs">
              {bot.revoked ? (
                <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-ink/70">
                  revoked
                </span>
              ) : (
                <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2 py-1 text-emerald-200/90">
                  active
                </span>
              )}
              <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-ink/70">
                offers: {data.offers_total}
              </span>
              <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-ink/70">
                jobs: {data.jobs_total}
              </span>
              <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-ink/70">
                payloads pending: {data.payloads_pending}
              </span>
            </div>
            <div className="mt-4 grid gap-2 text-sm text-ink/70 sm:grid-cols-2">
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  created_at
                </div>
                <div className="mt-1 font-mono">{bot.created_at}</div>
              </div>
              <div>
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  last_seen_at
                </div>
                <div className="mt-1 font-mono">{bot.last_seen_at ?? "--"}</div>
              </div>
              <div className="sm:col-span-2">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  signing_pubkey_ed25519
                </div>
                <div className="mt-1 break-all font-mono text-xs text-ink/70">
                  {bot.signing_pubkey_ed25519}
                </div>
              </div>
              <div className="sm:col-span-2">
                <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
                  encryption_pubkey_x25519
                </div>
                <div className="mt-1 break-all font-mono text-xs text-ink/70">
                  {bot.encryption_pubkey_x25519}
                </div>
              </div>
            </div>
          </div>

          <div className="flex flex-col gap-2 sm:items-end">
            <Link
              href={{
                pathname: "/admin/offers",
                query: { seller_bot_id: bot.bot_id },
              }}
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
            >
              View offers
            </Link>
            <Link
              href={{
                pathname: "/admin/jobs",
                query: { buyer_bot_id: bot.bot_id },
              }}
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
            >
              Jobs as buyer
            </Link>
            <Link
              href={{
                pathname: "/admin/jobs",
                query: { seller_bot_id: bot.bot_id },
              }}
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
            >
              Jobs as seller
            </Link>
          </div>
        </div>
      </div>

      <div className="glass-panel rounded-2xl p-6">
        <h2 className="text-base font-semibold text-ink">Moderation</h2>
        <p className="mt-1 text-sm text-ink/60">
          Revoking a bot cancels its ACTIVE/PAUSED offers and cancels REQUESTED /
          CHARGE_CREATED jobs tied to it. This is irreversible.
        </p>

        <form action={revokeAction} className="mt-4 grid gap-3 sm:grid-cols-2">
          <div className="sm:col-span-2">
            <label className="text-xs uppercase tracking-[0.28em] text-ink/60">
              Reason (required)
            </label>
            <input
              name="reason"
              required
              className="mt-1 h-10 w-full rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40"
              placeholder="why are we revoking this bot?"
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
              Type REVOKE to confirm
            </label>
            <input
              name="confirm"
              required
              className="mt-1 h-10 w-full rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40"
              placeholder="REVOKE"
            />
          </div>
          <div className="flex items-end">
            <button
              type="submit"
              className="h-10 w-full rounded-xl border border-rose-400/30 bg-rose-500/10 px-4 text-sm font-medium text-rose-100 hover:bg-rose-500/20"
              disabled={bot.revoked}
            >
              Revoke bot
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
