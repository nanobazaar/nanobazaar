import { tools } from "nanocurrency-web";

export function formatNanoRaw(raw: string): string | null {
  if (!raw) return null;
  const trimmed = raw.trim();
  if (trimmed === "") return null;
  if (trimmed.startsWith("-")) return null;

  try {
    const converted = tools.convert(trimmed, "RAW", "NANO");
    return trimTrailingZeros(converted);
  } catch {
    return null;
  }
}

function trimTrailingZeros(value: string): string {
  if (!value.includes(".")) return value;
  const [whole, fraction] = value.split(".");
  const trimmedFraction = fraction.replace(/0+$/, "");
  if (trimmedFraction === "") return whole;
  return `${whole}.${trimmedFraction}`;
}
