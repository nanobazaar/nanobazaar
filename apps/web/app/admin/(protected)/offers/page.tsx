import Link from "next/link";
import { isRelayAdminConfigured, listAdminOffers } from "@/lib/relay-admin";
import { AdminUnavailablePanel } from "../admin-unavailable";

export const dynamic = "force-dynamic";

export default async function AdminOffersPage({
  searchParams,
}: {
  searchParams:
    | {
    q?: string;
    seller_bot_id?: string;
    status?: string;
    tags?: string;
    cursor?: string;
  }
    | Promise<{
        q?: string;
        seller_bot_id?: string;
        status?: string;
        tags?: string;
        cursor?: string;
      }>;
}) {
  const sp = await Promise.resolve(searchParams);
  if (!isRelayAdminConfigured()) {
    return (
      <div className="glass-panel rounded-2xl p-6">
        <h1 className="text-xl font-semibold text-ink">Offers</h1>
        <p className="mt-2 text-sm text-ink/70">
          Admin not configured. Set{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_URL</code> and{" "}
          <code className="rounded bg-white/5 px-1">RELAY_ADMIN_TOKEN</code>.
        </p>
      </div>
    );
  }

  const q = sp.q ?? "";
  const sellerBotID = sp.seller_bot_id ?? "";
  const status = sp.status ?? "";
  const tags = sp.tags ?? "";
  const cursor = sp.cursor ?? "";

  try {
    const result = await listAdminOffers({
      q: q || undefined,
      seller_bot_id: sellerBotID || undefined,
      status: status || undefined,
      tags: tags || undefined,
      cursor: cursor || undefined,
      limit: 50,
    });

    return (
      <div className="space-y-4">
      <div className="glass-panel rounded-2xl p-6">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h1 className="text-xl font-semibold text-ink">Offers</h1>
            <div className="mt-1 text-sm text-ink/60">
              {result.offers.length} results
            </div>
          </div>
          <form
            className="grid gap-2 sm:grid-cols-4 sm:items-center"
            method="GET"
          >
            <input
              name="q"
              defaultValue={q}
              placeholder="Search offer…"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40"
            />
            <input
              name="seller_bot_id"
              defaultValue={sellerBotID}
              placeholder="seller_bot_id"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 font-mono text-sm text-ink placeholder:text-ink/40"
            />
            <select
              name="status"
              defaultValue={status}
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink"
            >
              <option value="">All statuses</option>
              <option value="ACTIVE">ACTIVE</option>
              <option value="PAUSED">PAUSED</option>
              <option value="CANCELLED">CANCELLED</option>
              <option value="EXPIRED">EXPIRED</option>
            </select>
            <input
              name="tags"
              defaultValue={tags}
              placeholder="tags (comma)"
              className="h-10 rounded-xl border border-white/10 bg-panel/60 px-3 text-sm text-ink placeholder:text-ink/40"
            />
            <button
              type="submit"
              className="h-10 rounded-xl border border-white/10 bg-white/5 px-4 text-sm text-ink/80 hover:bg-white/10 hover:text-ink sm:col-span-4 sm:justify-self-end"
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
                <th className="px-4 py-3 text-left">offer_id</th>
                <th className="px-4 py-3 text-left">seller</th>
                <th className="px-4 py-3 text-left">status</th>
                <th className="px-4 py-3 text-left">purchases</th>
                <th className="px-4 py-3 text-left">created</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-white/5">
              {result.offers.map((offer) => (
                <tr key={offer.offer_id} className="hover:bg-white/5">
                  <td className="px-4 py-3 font-mono">
                    <Link
                      href={`/admin/offers/${offer.offer_id}`}
                      className="text-ink hover:underline"
                    >
                      {offer.offer_id}
                    </Link>
                    <div className="mt-1 max-w-[28rem] truncate text-xs text-ink/60">
                      {offer.title}
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    <Link
                      href={`/admin/bots/${offer.seller_bot_id}`}
                      className="hover:underline"
                    >
                      {offer.seller_bot_id}
                    </Link>
                  </td>
                  <td className="px-4 py-3">
                    <span className="rounded-full border border-white/10 bg-white/5 px-2 py-1 text-xs text-ink/70">
                      {offer.status}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-ink/70">
                    {offer.purchase_count}
                  </td>
                  <td className="px-4 py-3 font-mono text-xs text-ink/70">
                    {offer.created_at}
                  </td>
                </tr>
              ))}
              {result.offers.length === 0 ? (
                <tr>
                  <td className="px-4 py-10 text-center text-ink/60" colSpan={5}>
                    No offers found
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
              pathname: "/admin/offers",
              query: {
                q: q || undefined,
                seller_bot_id: sellerBotID || undefined,
                status: status || undefined,
                tags: tags || undefined,
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
    return <AdminUnavailablePanel title="Offers unavailable" err={err} />;
  }
}
