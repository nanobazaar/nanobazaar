import Link from "next/link";
import { isRelayAdminConfigured, listAdminPayloads } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../admin-unavailable";

export const dynamic = "force-dynamic";

function formatBytes(bytes: number) {
  if (!Number.isFinite(bytes) || bytes <= 0) return "0";
  const units = ["B", "KB", "MB", "GB"];
  let idx = 0;
  let v = bytes;
  while (v >= 1024 && idx < units.length - 1) {
    v /= 1024;
    idx += 1;
  }
  const decimals = idx === 0 ? 0 : v < 10 ? 2 : 1;
  return `${v.toFixed(decimals)} ${units[idx]}`;
}

export default async function AdminPayloadsPage({
  searchParams,
}: {
  searchParams:
    | {
        status?: string;
        job_id?: string;
        recipient_bot_id?: string;
        sender_bot_id?: string;
        cursor?: string;
      }
    | Promise<{
        status?: string;
        job_id?: string;
        recipient_bot_id?: string;
        sender_bot_id?: string;
        cursor?: string;
      }>;
}) {
  const sp = await Promise.resolve(searchParams);
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Payloads</h1>
        <p className="mt-2 text-sm text-ink/70">
          Admin not configured. Set{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code> and{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code>.
        </p>
      </div>
    );
  }

  const status =
    sp.status === "fetched" || sp.status === "all"
      ? (sp.status as "fetched" | "all")
      : ("unfetched" as const);
  const jobId = sp.job_id ?? "";
  const recipientBotID = sp.recipient_bot_id ?? "";
  const senderBotID = sp.sender_bot_id ?? "";
  const cursor = sp.cursor ?? "";

  try {
    const result = await listAdminPayloads({
      status,
      job_id: jobId || undefined,
      recipient_bot_id: recipientBotID || undefined,
      sender_bot_id: senderBotID || undefined,
      cursor: cursor || undefined,
      limit: 50,
    });

    return (
      <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-ink">Payloads</h1>
            <div className="mt-1 text-sm text-ink/60">
              {result.payloads.length} results
            </div>
          </div>
          <form className="grid gap-2 sm:grid-cols-5" method="GET">
            <select
              name="status"
              defaultValue={status}
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink"
            >
              <option value="unfetched">unfetched</option>
              <option value="fetched">fetched</option>
              <option value="all">all</option>
            </select>
            <input
              name="job_id"
              defaultValue={jobId}
              placeholder="job_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <input
              name="recipient_bot_id"
              defaultValue={recipientBotID}
              placeholder="recipient_bot_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <input
              name="sender_bot_id"
              defaultValue={senderBotID}
              placeholder="sender_bot_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <div className="sm:col-span-3 sm:flex sm:justify-end">
              <button
                type="submit"
                className="h-10 w-full rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink sm:w-auto"
              >
                Filter
              </button>
            </div>
          </form>
        </div>
      </div>

      <div className="glass-panel overflow-hidden rounded-2xl">
        <div className="overflow-x-auto">
          <table className="min-w-full text-sm">
            <thead className="bg-panel/60 text-xs uppercase tracking-[0.22em] text-ink/60">
              <tr>
                <th className="px-4 py-3 text-left">payload_id</th>
                <th className="px-4 py-3 text-left">job</th>
                <th className="px-4 py-3 text-left">kind</th>
                <th className="px-4 py-3 text-left">sender</th>
                <th className="px-4 py-3 text-left">recipient</th>
                <th className="px-4 py-3 text-left">created</th>
                <th className="px-4 py-3 text-left">size</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {result.payloads.map((p) => (
                <tr key={`${p.payload_id}:${p.recipient_bot_id}`} className="hover:bg-white/5">
                  <td className="px-4 py-3 font-mono text-xs text-ink/80">
                    {p.payload_id}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    <Link
                      href={`/admin/jobs/${p.job_id}`}
                      className="hover:underline"
                    >
                      {p.job_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink/70">{p.payload_kind}</td>
                  <td className="px-4 py-3 text-xs text-ink/70">
                    <Link
                      href={`/admin/bots/${p.sender_bot_id}`}
                      className="block hover:underline"
                    >
                      {p.sender_bot_name ? (
                        <div className="leading-tight">
                          <div className="font-medium text-ink/80">
                            {p.sender_bot_name}
                          </div>
                          <div className="mt-0.5 font-mono text-[10px] text-ink/60">
                            {p.sender_bot_id}
                          </div>
                        </div>
                      ) : (
                        <span className="font-mono">{p.sender_bot_id}</span>
                      )}
                    </Link>
                  </td>
                  <td className="px-4 py-3 text-xs text-ink/70">
                    <Link
                      href={`/admin/bots/${p.recipient_bot_id}`}
                      className="block hover:underline"
                    >
                      {p.recipient_bot_name ? (
                        <div className="leading-tight">
                          <div className="font-medium text-ink/80">
                            {p.recipient_bot_name}
                          </div>
                          <div className="mt-0.5 font-mono text-[10px] text-ink/60">
                            {p.recipient_bot_id}
                          </div>
                        </div>
                      ) : (
                        <span className="font-mono">{p.recipient_bot_id}</span>
                      )}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    {p.created_at}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    {formatBytes(p.ciphertext_b64_bytes)}
                  </td>
                </tr>
              ))}
              {result.payloads.length === 0 ? (
                <tr>
                  <td className="px-4 py-10 text-center text-ink/60" colSpan={7}>
                    No payloads found
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
              pathname: "/admin/payloads",
              query: {
                status,
                job_id: jobId || undefined,
                recipient_bot_id: recipientBotID || undefined,
                sender_bot_id: senderBotID || undefined,
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
    return <AdminUnavailablePanel title="Payloads unavailable" err={err} />;
  }
}
