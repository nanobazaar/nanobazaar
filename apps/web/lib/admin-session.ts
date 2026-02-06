import "server-only";

import { createHmac, timingSafeEqual } from "crypto";

type SessionPayload = {
  sub: string;
  iat: number;
  exp: number;
};

function base64urlEncode(input: Buffer): string {
  return input
    .toString("base64")
    .replace(/\+/g, "-")
    .replace(/\//g, "_")
    .replace(/=+$/g, "");
}

function base64urlDecode(input: string): Buffer | null {
  const normalized = input.replace(/-/g, "+").replace(/_/g, "/");
  const pad = normalized.length % 4 === 0 ? "" : "=".repeat(4 - (normalized.length % 4));
  try {
    return Buffer.from(normalized + pad, "base64");
  } catch {
    return null;
  }
}

function hmacSHA256(secret: string, msg: string): Buffer {
  return createHmac("sha256", secret).update(msg).digest();
}

export function signAdminSession(payload: SessionPayload, secret: string): string {
  const body = base64urlEncode(Buffer.from(JSON.stringify(payload), "utf8"));
  const sig = base64urlEncode(hmacSHA256(secret, body));
  return `${body}.${sig}`;
}

export function verifyAdminSession(cookieValue: string, secret: string): SessionPayload | null {
  const parts = cookieValue.split(".");
  if (parts.length !== 2) return null;
  const [bodyB64, sigB64] = parts;

  const expectedSig = base64urlEncode(hmacSHA256(secret, bodyB64));
  if (sigB64.length !== expectedSig.length) return null;
  if (
    timingSafeEqual(Buffer.from(sigB64, "utf8"), Buffer.from(expectedSig, "utf8")) === false
  ) {
    return null;
  }

  const decoded = base64urlDecode(bodyB64);
  if (!decoded) return null;
  let payload: any;
  try {
    payload = JSON.parse(decoded.toString("utf8"));
  } catch {
    return null;
  }

  if (!payload || typeof payload !== "object") return null;
  if (typeof payload.sub !== "string") return null;
  if (typeof payload.iat !== "number") return null;
  if (typeof payload.exp !== "number") return null;
  if (payload.exp * 1000 < Date.now()) return null;

  return payload as SessionPayload;
}

