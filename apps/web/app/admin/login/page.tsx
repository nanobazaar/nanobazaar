export const dynamic = "force-dynamic";

import { cookies } from "next/headers";
import { redirect } from "next/navigation";
import { createHash, timingSafeEqual } from "crypto";

import { signAdminSession } from "../../../lib/admin-session";
import { getAdminSession } from "../../../lib/admin-auth";

function mustEnv(name: string): string {
  const value = process.env[name];
  if (!value) throw new Error(`Missing required env var: ${name}`);
  return value;
}

function sha256Hex(input: string): string {
  return createHash("sha256").update(input, "utf8").digest("hex");
}

function safeEqualString(a: string, b: string): boolean {
  const ah = Buffer.from(sha256Hex(a), "utf8");
  const bh = Buffer.from(sha256Hex(b), "utf8");
  return timingSafeEqual(ah, bh);
}

async function loginAction(formData: FormData) {
  "use server";

  const expectedUser = process.env.ADMIN_LOGIN_USERNAME || "admin";
  const expectedPass = mustEnv("ADMIN_LOGIN_PASSWORD");
  const sessionSecret = mustEnv("ADMIN_SESSION_SECRET");

  const user = String(formData.get("username") || "");
  const pass = String(formData.get("password") || "");

  const okUser = safeEqualString(user, expectedUser);
  const okPass = safeEqualString(pass, expectedPass);
  if (!okUser || !okPass) {
    redirect("/admin/login?error=1");
  }

  const now = Math.floor(Date.now() / 1000);
  const ttlSeconds = Number(process.env.ADMIN_SESSION_TTL_SECONDS || "604800"); // 7d
  const payload = { sub: expectedUser, iat: now, exp: now + ttlSeconds };
  const token = signAdminSession(payload, sessionSecret);

  const cookieStore = await cookies();
  cookieStore.set("nbr_admin_session", token, {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/admin",
    maxAge: ttlSeconds
  });

  redirect("/admin");
}

export default async function AdminLoginPage({
  searchParams
}: {
  searchParams?: { error?: string };
}) {
  if (await getAdminSession()) redirect("/admin");

  const showError = searchParams?.error === "1";

  return (
    <main className="min-h-screen bg-neutral-950 text-neutral-50">
      <div className="mx-auto flex min-h-screen w-full max-w-md flex-col justify-center px-6 py-10">
        <h1 className="text-2xl font-semibold tracking-tight">Relay Admin</h1>
        <p className="mt-2 text-sm text-neutral-300">
          Sign in to access the moderation and operations dashboard.
        </p>

        {showError ? (
          <div className="mt-6 rounded-md border border-red-500/30 bg-red-500/10 px-3 py-2 text-sm text-red-200">
            Invalid credentials
          </div>
        ) : null}

        <form action={loginAction} className="mt-8 space-y-4">
          <label className="block">
            <div className="text-xs font-medium text-neutral-200">Username</div>
            <input
              name="username"
              autoComplete="username"
              defaultValue="admin"
              className="mt-2 w-full rounded-md border border-white/10 bg-white/5 px-3 py-2 text-sm outline-none placeholder:text-neutral-500 focus:border-white/20 focus:bg-white/10"
            />
          </label>

          <label className="block">
            <div className="text-xs font-medium text-neutral-200">Password</div>
            <input
              type="password"
              name="password"
              autoComplete="current-password"
              className="mt-2 w-full rounded-md border border-white/10 bg-white/5 px-3 py-2 text-sm outline-none placeholder:text-neutral-500 focus:border-white/20 focus:bg-white/10"
            />
          </label>

          <button
            type="submit"
            className="w-full rounded-md bg-white px-3 py-2 text-sm font-medium text-neutral-950 hover:bg-neutral-200"
          >
            Sign in
          </button>

          <p className="text-xs text-neutral-400">
            Uses an encrypted, httpOnly session cookie (expires after{" "}
            {Number(process.env.ADMIN_SESSION_TTL_SECONDS || "604800") / 86400} days).
          </p>
        </form>
      </div>
    </main>
  );
}
