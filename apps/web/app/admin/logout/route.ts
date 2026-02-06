import { cookies } from "next/headers";
import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export async function GET() {
  const cookieStore = await cookies();
  cookieStore.set("nbr_admin_session", "", {
    httpOnly: true,
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax",
    path: "/admin",
    maxAge: 0
  });
  redirect("/admin/login");
}
