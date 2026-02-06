import Link from "next/link";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { getAdminSession } from "../../../lib/admin-auth";

const nav = [
  { href: "/admin", label: "Overview" },
  { href: "/admin/bots", label: "Bots" },
  { href: "/admin/offers", label: "Offers" },
  { href: "/admin/jobs", label: "Jobs" },
  { href: "/admin/payloads", label: "Payloads" },
  { href: "/admin/audit", label: "Audit" }
];

export default async function AdminLayout({ children }: { children: ReactNode }) {
  if (!(await getAdminSession())) {
    redirect("/admin/login");
  }

  return (
    <div className="mx-auto w-full max-w-6xl px-4 pb-20 pt-8 sm:px-6">
      <div className="glass-panel rounded-2xl px-4 py-3">
        <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <div className="text-xs uppercase tracking-[0.3em] text-ink/60">
              NanoBazaar
            </div>
            <div className="text-lg font-semibold text-ink">Relay Admin</div>
          </div>
          <nav className="flex flex-wrap gap-2 text-sm">
            {nav.map((item) => (
              <Link
                key={item.href}
                href={item.href}
                prefetch={false}
                className="rounded-full border border-white/10 bg-panel/40 px-3 py-1.5 text-ink/80 transition hover:bg-panel/70 hover:text-ink"
              >
                {item.label}
              </Link>
            ))}
            <Link
              href="/admin/logout"
              prefetch={false}
              className="rounded-full border border-white/10 bg-panel/20 px-3 py-1.5 text-ink/70 transition hover:bg-panel/40 hover:text-ink"
            >
              Logout
            </Link>
          </nav>
        </div>
      </div>
      <div className="mt-6">{children}</div>
    </div>
  );
}
