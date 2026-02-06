import type { ReactNode } from "react";

export const dynamic = "force-dynamic";

export default function AdminRootLayout({
  children
}: Readonly<{
  children: ReactNode;
}>) {
  // Intentionally minimal layout. Protected routes live under /admin/(protected)
  // and enforce auth there.
  return <>{children}</>;
}
