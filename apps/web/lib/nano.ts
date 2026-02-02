import { tools } from "nanocurrency-web";

export function formatNanoRaw(raw: string): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  if (trimmed.startsWith("-")) return null;

  try {
    return tools.convert(trimmed, "RAW", "NANO");
  } catch {
    return null;
  }
}
