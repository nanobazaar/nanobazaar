import Link from "next/link";
import { isRelayAdminConfigured, listAdminJobs } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../admin-unavailable";

export const dynamic = "force-dynamic";

export default async function AdminJobsPage({
  searchParams,
}: {
  searchParams:
    | {
    q?: string;
    offer_id?: string;
    buyer_bot_id?: string;
    seller_bot_id?: string;
    status?: string;
    created_since?: string;
    cursor?: string;
  }
    | Promise<{
        q?: string;
        offer_id?: string;
        buyer_bot_id?: string;
        seller_bot_id?: string;
        status?: string;
        created_since?: string;
        cursor?: string;
      }>;
}) {
  const sp = await Promise.resolve(searchParams);
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Jobs</h1>
        <p className="mt-2 text-sm text-ink/70">
          Admin not configured. Set{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code> and{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code>.
        </p>
      </div>
    );
  }

  const q = sp.q ?? "";
  const offerId = sp.offer_id ?? "";
  const buyerBotID = sp.buyer_bot_id ?? "";
  const sellerBotID = sp.seller_bot_id ?? "";
  const status = sp.status ?? "";
  const createdSince = sp.created_since ?? "";
  const cursor = sp.cursor ?? "";

  try {
    const result = await listAdminJobs({
      q: q || undefined,
      offer_id: offerId || undefined,
      buyer_bot_id: buyerBotID || undefined,
      seller_bot_id: sellerBotID || undefined,
      created_since: createdSince || undefined,
      cursor: cursor || undefined,
      status: status ? [status] : undefined,
      limit: 50,
    });

    return (
      <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-ink">Jobs</h1>
            <div className="mt-1 text-sm text-ink/60">
              {result.jobs.length} results
            </div>
          </div>
          <form className="grid gap-2 sm:grid-cols-6" method="GET">
            <input
              name="q"
              defaultValue={q}
              placeholder="job_id…"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <input
              name="offer_id"
              defaultValue={offerId}
              placeholder="offer_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-2"
            />
            <select
              name="status"
              defaultValue={status}
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink"
            >
              <option value="">All statuses</option>
              <option value="REQUESTED">REQUESTED</option>
              <option value="CHARGE_CREATED">CHARGE_CREATED</option>
              <option value="PAID">PAID</option>
              <option value="DELIVERED">DELIVERED</option>
              <option value="CANCELLED">CANCELLED</option>
              <option value="EXPIRED">EXPIRED</option>
            </select>
            <input
              name="created_since"
              defaultValue={createdSince}
              placeholder="created_since (RFC3339Nano)"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40 sm:col-span-3"
            />
            <input
              name="buyer_bot_id"
              defaultValue={buyerBotID}
              placeholder="buyer_bot_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-3"
            />
            <input
              name="seller_bot_id"
              defaultValue={sellerBotID}
              placeholder="seller_bot_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40 sm:col-span-3"
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
                <th className="px-4 py-3 text-left">job_id</th>
                <th className="px-4 py-3 text-left">status</th>
                <th className="px-4 py-3 text-left">buyer</th>
                <th className="px-4 py-3 text-left">seller</th>
                <th className="px-4 py-3 text-left">created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {result.jobs.map((job) => (
                <tr key={job.job_id} className="hover:bg-white/5">
                  <td className="px-4 py-3 font-mono">
                    <Link
                      href={`/admin/jobs/${job.job_id}`}
                      className="text-ink hover:underline"
                    >
                      {job.job_id}
                    </Link>
                    <div className="mt-1 font-mono text-xs text-ink/60">
                      offer:{" "}
                      <Link
                        href={`/admin/offers/${job.offer_id}`}
                        className="hover:underline"
                      >
                        {job.offer_id}
                      </Link>
                    </div>
                  </td>
                  <td className="px-4 py-3">
                    <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
                      {job.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    <Link
                      href={`/admin/bots/${job.buyer_bot_id}`}
                      className="hover:underline"
                    >
                      {job.buyer_bot_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    <Link
                      href={`/admin/bots/${job.seller_bot_id}`}
                      className="hover:underline"
                    >
                      {job.seller_bot_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    {job.created_at}
                  </td>
                </tr>
              ))}
              {result.jobs.length === 0 ? (
                <tr>
                  <td className="px-4 py-10 text-center text-ink/60" colSpan={5}>
                    No jobs found
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
              pathname: "/admin/jobs",
              query: {
                q: q || undefined,
                offer_id: offerId || undefined,
                buyer_bot_id: buyerBotID || undefined,
                seller_bot_id: sellerBotID || undefined,
                status: status || undefined,
                created_since: createdSince || undefined,
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
    return <AdminUnavailablePanel title="Jobs unavailable" err={err} />;
  }
}
