import type { ReactNode } from "react";
import { cn } from "@/lib/utils";

export function TiltCard({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={cn("min-w-0 [overflow-wrap:anywhere]", className)}>{children}</div>;
}
