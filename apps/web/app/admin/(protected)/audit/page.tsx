import Link from "next/link";
import { isRelayAdminConfigured, listAdminAudit } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../admin-unavailable";

export const dynamic = "force-dynamic";

function truncate(value: string, max: number) {
  if (value.length <= max) return value;
  return value.slice(0, max - 1) + "…";
}

export default async function AdminAuditPage({
  searchParams,
}: {
  searchParams:
    | {
        target_type?: string;
        target_id?: string;
        cursor?: string;
      }
    | Promise<{
        target_type?: string;
        target_id?: string;
        cursor?: string;
      }>;
}) {
  const sp = await Promise.resolve(searchParams);
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Audit</h1>
        <p className="mt-2 text-sm text-ink/70">
          Admin not configured. Set{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code> and{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code>.
        </p>
      </div>
    );
  }

  const targetType = sp.target_type ?? "";
  const targetId = sp.target_id ?? "";
  const cursor = sp.cursor ?? "";

  try {
    const result = await listAdminAudit({
      target_type: targetType || undefined,
      target_id: targetId || undefined,
      cursor: cursor || undefined,
      limit: 50,
    });

    return (
      <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-ink">Audit Log</h1>
            <div className="mt-1 text-sm text-ink/60">
              {result.entries.length} entries
            </div>
          </div>
          <form className="grid gap-2 sm:grid-cols-3" method="GET">
            <input
              name="target_type"
              defaultValue={targetType}
              placeholder="target_type (bot|offer|job)"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40"
            />
            <input
              name="target_id"
              defaultValue={targetId}
              placeholder="target_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <button
              type="submit"
              className="h-10 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink sm:col-span-3 sm:justify-self-end"
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
                <th className="px-4 py-3 text-left">when</th>
                <th className="px-4 py-3 text-left">action</th>
                <th className="px-4 py-3 text-left">target</th>
                <th className="px-4 py-3 text-left">reason</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {result.entries.map((e) => (
                <tr key={e.id} className="hover:bg-white/5">
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    {e.created_at}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/80">
                    {e.action}
                  </td>
                  <td className="px-4 py-3 text-xs text-ink/70">
                    <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 font-mono text-xs">
                      {e.target_type}:{truncate(e.target_id, 32)}
                    </span>
                    {e.target_type === "bot" ? (
                      <Link
                        className="ml-2 text-ink/70 hover:underline"
                        href={`/admin/bots/${e.target_id}`}
                      >
                        open
                      </Link>
                    ) : null}
                    {e.target_type === "offer" ? (
                      <Link
                        className="ml-2 text-ink/70 hover:underline"
                        href={`/admin/offers/${e.target_id}`}
                      >
                        open
                      </Link>
                    ) : null}
                    {e.target_type === "job" ? (
                      <Link
                        className="ml-2 text-ink/70 hover:underline"
                        href={`/admin/jobs/${e.target_id}`}
                      >
                        open
                      </Link>
                    ) : null}
                  </td>
                  <td className="px-4 py-3 text-xs text-ink/70">
                    {truncate(e.reason, 140)}
                  </td>
                </tr>
              ))}
              {result.entries.length === 0 ? (
                <tr>
                  <td className="px-4 py-10 text-center text-ink/60" colSpan={4}>
                    No audit entries
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
              pathname: "/admin/audit",
              query: {
                target_type: targetType || undefined,
                target_id: targetId || undefined,
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
    return <AdminUnavailablePanel title="Audit unavailable" err={err} />;
  }
}
