import "server-only";

import { cookies } from "next/headers";

import { verifyAdminSession } from "./admin-session";

export type AdminSession = {
  sub: string;
  iat: number;
  exp: number;
};

function getSessionSecret(): string | null {
  return process.env.ADMIN_SESSION_SECRET || null;
}

export async function getAdminSession(): Promise<AdminSession | null> {
  const secret = getSessionSecret();
  if (!secret) return null;

  const cookieStore = await cookies();
  const value = cookieStore.get("nbr_admin_session")?.value;
  if (!value) return null;

  return verifyAdminSession(value, secret);
}

export async function requireAdminSession(): Promise<AdminSession> {
  const session = await getAdminSession();
  if (!session) throw new Error("Not authenticated");
  return session;
}
