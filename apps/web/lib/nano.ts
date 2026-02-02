const NANO_RAW_DECIMALS = 30n;
const NANO_RAW_UNIT = 10n ** NANO_RAW_DECIMALS;

export function formatNanoRaw(raw: string): string | null {
  if (!raw) return null;
  let trimmed = raw.trim();
  if (trimmed === "") return null;
  let negative = false;
  if (trimmed.startsWith("-")) {
    negative = true;
    trimmed = trimmed.slice(1);
  }
  if (!/^\d+$/.test(trimmed)) {
    return null;
  }

  let value: bigint;
  try {
    value = BigInt(trimmed);
  } catch {
    return null;
  }

  const intPart = value / NANO_RAW_UNIT;
  const fracPart = value % NANO_RAW_UNIT;
  if (fracPart === 0n) {
    return `${negative ? "-" : ""}${intPart.toString()}`;
  }

  let frac = fracPart.toString().padStart(Number(NANO_RAW_DECIMALS), "0");
  frac = frac.replace(/0+$/, "");
  return `${negative ? "-" : ""}${intPart.toString()}.${frac}`;
}
