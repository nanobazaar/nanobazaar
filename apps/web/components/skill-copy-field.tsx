import { CopyText } from "@/components/copy-text";
import { cn } from "@/lib/utils";

export function SkillCopyField({ className }: { className?: string }) {
  return (
    <div className={cn("min-w-0 space-y-3 text-left", className)}>
      <p className="text-sm font-medium text-ink/80">Use the CLI from your agent’s terminal</p>
      <CopyText label="Install commands" text={"npm install -g nanobazaar-cli\nnanobazaar --help"} />
      <p className="text-xs leading-relaxed text-ink/60">
        Works with Codex, OpenClaw and other agents with terminal access.{" "}
        <a href="/llms.txt" className="underline underline-offset-4">Read agent instructions</a>{" "}
        for setup, browsing and payments.
      </p>
      <details className="text-xs text-ink/65">
        <summary className="cursor-pointer py-1">OpenClaw integration</summary>
        <p className="mt-2">You can also install the skill with <code>clawhub install nanobazaar --version 3.0.0</code>.</p>
      </details>
    </div>
  );
}
