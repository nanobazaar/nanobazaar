import Link from "next/link";
import { isRelayAdminConfigured, listAdminBots } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../admin-unavailable";

export const dynamic = "force-dynamic";

function formatNumber(value: number) {
  return new Intl.NumberFormat("en-US").format(value);
}

export default async function AdminBotsPage({
  searchParams,
}: {
  searchParams: { q?: string; revoked?: string; cursor?: string } | Promise<{ q?: string; revoked?: string; cursor?: string }>;
}) {
  const sp = await Promise.resolve(searchParams);
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Bots</h1>
        <p className="mt-2 text-sm text-ink/70">
          Admin not configured. Set <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code>{" "}
          and <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code>.
        </p>
      </div>
    );
  }

  const q = sp.q ?? "";
  const revoked = sp.revoked ?? "";
  const cursor = sp.cursor ?? "";

  try {
    const result = await listAdminBots({
      q: q || undefined,
      revoked: revoked || undefined,
      cursor: cursor || undefined,
      limit: 50,
    });

    return (
      <div className="space-y-4">
        <div className="glass-panel rounded-2xl p-6">
          <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
            <div>
              <h1 className="text-xl font-semibold text-ink">Bots</h1>
              <div className="mt-1 text-sm text-ink/60">
                Showing {formatNumber(result.bots.length)} bots
              </div>
            </div>
            <form className="flex flex-col gap-2 sm:flex-row sm:items-center" method="GET">
              <input
                name="q"
                defaultValue={q}
                placeholder="Search bot_id…"
                className="h-10 w-full rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40 sm:w-64"
              />
              <select
                name="revoked"
                defaultValue={revoked}
                className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink"
              >
                <option value="">All</option>
                <option value="false">Active</option>
                <option value="true">Revoked</option>
              </select>
              <button
                type="submit"
                className="h-10 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
              >
                Filter
              </button>
            </form>
          </div>
        </div>

        <div className="glass-panel overflow-hidden rounded-2xl">
          <div className="overflow-x-auto">
            <table className="min-w-full text-sm">
              <thead className="bg-panel/60 text-xs uppercase tracking-[0.22em] text-ink/60">
                <tr>
                  <th className="px-4 py-3 text-left">bot_id</th>
                  <th className="px-4 py-3 text-left">created</th>
                  <th className="px-4 py-3 text-left">last_seen</th>
                  <th className="px-4 py-3 text-left">status</th>
                </tr>
              </thead>
              <tbody className="divide-y divide-white/5">
                {result.bots.map((bot) => (
                  <tr key={bot.bot_id} className="hover:bg-white/5">
                    <td className="px-4 py-3 font-mono">
                      <Link
                        href={`/admin/bots/${bot.bot_id}`}
                        className="text-ink hover:underline"
                      >
                        {bot.bot_id}
                      </Link>
                    </td>
                    <td className="px-4 py-3 font-mono text-ink/70">
                      {bot.created_at}
                    </td>
                    <td className="px-4 py-3 font-mono text-ink/70">
                      {bot.last_seen_at ?? "--"}
                    </td>
                    <td className="px-4 py-3">
                      {bot.revoked ? (
                        <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
                          revoked
                        </span>
                      ) : (
                        <span className="rounded-full border border-emerald-400/20 bg-emerald-400/10 px-2 py-1 text-xs text-emerald-200/90">
                          active
                        </span>
                      )}
                    </td>
                  </tr>
                ))}
                {result.bots.length === 0 ? (
                  <tr>
                    <td className="px-4 py-10 text-center text-ink/60" colSpan={4}>
                      No bots found
                    </td>
                  </tr>
                ) : null}
              </tbody>
            </table>
          </div>
        </div>

        {result.next_cursor ? (
          <div className="flex justify-end">
            <Link
              className="rounded-xl border border-white/10 bg-white/5 px-4 py-2 text-sm text-ink/80 hover:bg-white/10 hover:text-ink"
              href={{
                pathname: "/admin/bots",
                query: {
                  q: q || undefined,
                  revoked: revoked || undefined,
                  cursor: result.next_cursor,
                },
              }}
            >
              Next page
            </Link>
          </div>
        ) : null}
      </div>
    );
  } catch (err) {
    return <AdminUnavailablePanel title="Bots unavailable" err={err} />;
  }
}
