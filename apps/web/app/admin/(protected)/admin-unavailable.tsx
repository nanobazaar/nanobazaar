import { RelayAdminError } from "@/lib/relay-admin";

export function AdminUnavailablePanel({
  title,
  err
}: {
  title: string;
  err: unknown;
}) {
  let message = "Unknown error";
  let hint: string | null = null;
  let status: number | null = null;

  if (err instanceof RelayAdminError) {
    message = err.message;
    status = err.status;

    if (err.status === 404) {
      hint =
        "The relay returned 404 for /admin/*, which usually means admin routes are not mounted on that listener. If you're using Vercel, you almost certainly want NBR_ADMIN_PUBLIC=true on Fly.";
    } else if (err.status === 401) {
      hint =
        "Auth failed. Ensure RELAY_ADMIN_TOKEN in Vercel matches NBR_ADMIN_TOKEN on Fly, and that the request includes 'Authorization: Bearer ...'.";
    }
  } else if (err instanceof Error) {
    message = err.message;
    if (err.message === "fetch failed") {
      hint =
        "Network fetch failed from the Next.js server to the relay. For local dev, this is often IPv6 localhost resolution or the relay not running. Try RELAY_ADMIN_URL=http://127.0.0.1:8080 and restart pnpm dev, and confirm curl -H 'Authorization: Bearer ...' http://127.0.0.1:8080/admin/meta works.";
    }
  }

  return (
    <div className="glass-panel rounded-2xl p-6">
      <h1 className="text-xl font-semibold text-ink">{title}</h1>
      <div className="mt-2 text-sm text-ink/70">{message}</div>
      {status !== null ? (
        <div className="mt-2 text-xs text-ink/60">
          status: <span className="font-mono text-ink/70">{status}</span>
        </div>
      ) : null}

      {hint ? <div className="mt-4 text-sm text-ink/70">{hint}</div> : null}

      <div className="mt-5 rounded-2xl border border-white/10 bg-panel/50 p-4 text-sm text-ink/70">
        <div className="text-xs uppercase tracking-[0.28em] text-ink/60">
          Vercel Checklist
        </div>
        <ul className="mt-3 list-disc space-y-1 pl-5">
          <li>
            Fly: set <span className="font-mono">NBR_ADMIN_PUBLIC=true</span>
          </li>
          <li>
            Fly: set a strong <span className="font-mono">NBR_ADMIN_TOKEN</span>
          </li>
          <li>
            Vercel: set <span className="font-mono">RELAY_ADMIN_URL</span> to your
            relay HTTPS URL
          </li>
          <li>
            Vercel: set <span className="font-mono">RELAY_ADMIN_TOKEN</span> to
            match Fly
          </li>
        </ul>
      </div>
    </div>
  );
}
