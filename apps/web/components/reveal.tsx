import type { ReactNode } from "react";

// Essential content must be visible in the initial HTML, including without JS.
export function Reveal({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={className}>{children}</div>;
}
