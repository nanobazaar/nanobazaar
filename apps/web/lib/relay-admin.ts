import "server-only";

type Json = null | boolean | number | string | Json[] | { [key: string]: Json };

export class RelayAdminError extends Error {
  status: number;
  detail: unknown;

  constructor(message: string, status: number, detail: unknown) {
    super(message);
    this.name = "RelayAdminError";
    this.status = status;
    this.detail = detail;
  }
}

function resolveRelayAdminBaseUrl(): string | null {
  const base =
    process.env.RELAY_ADMIN_URL ||
    process.env.NBR_RELAY_ADMIN_URL ||
    process.env.NEXT_PUBLIC_RELAY_ADMIN_URL;
  if (!base) return null;
  return base.endsWith("/") ? base.slice(0, -1) : base;
}

function resolveRelayAdminToken(): string | null {
  return process.env.RELAY_ADMIN_TOKEN || process.env.NBR_ADMIN_TOKEN || null;
}

export function isRelayAdminConfigured(): boolean {
  return Boolean(resolveRelayAdminBaseUrl() && resolveRelayAdminToken());
}

async function adminFetch<T extends Json>(
  path: string,
  options: RequestInit & { body?: string } = {}
): Promise<T> {
  const baseUrl = resolveRelayAdminBaseUrl();
  const token = resolveRelayAdminToken();
  if (!baseUrl || !token) {
    throw new Error("Relay admin not configured");
  }

  const url = new URL(path, baseUrl);
  const headers = new Headers(options.headers);
  headers.set("Authorization", `Bearer ${token}`);
  if (!headers.has("Accept")) headers.set("Accept", "application/json");

  let response: Response;
  try {
    response = await fetch(url, {
      ...options,
      headers,
      cache: "no-store"
    });
  } catch (err) {
    const cause = typeof err === "object" && err && "cause" in err ? (err as any).cause : null;
    const causeMsg =
      cause instanceof Error ? cause.message : cause ? String(cause) : "";
    const msg = `Relay admin fetch failed (${url.toString()})${causeMsg ? `: ${causeMsg}` : ""}`;
    const e = new Error(msg);
    (e as any).cause = err;
    throw e;
  }

  const contentType = response.headers.get("content-type") || "";
  const isJson = contentType.includes("application/json");
  if (!response.ok) {
    const detail = isJson ? await response.json().catch(() => null) : null;
    const message =
      typeof detail === "object" && detail && "error" in detail
        ? String((detail as any).error)
        : `Relay admin request failed (${response.status})`;
    throw new RelayAdminError(message, response.status, detail);
  }

  if (!isJson) return null as T;
  return (await response.json()) as T;
}

export type AdminOverview = {
  now: string;
  bots: { active: number; revoked: number; total: number };
  offers_by_status: Record<string, number>;
  jobs_by_status: Record<string, number>;
  payloads: { pending: number; total: number; stored_bytes: number };
  events_total: number;
  stream: { active_conns: number; active_bots: number; active_streams: number };
  needs_attention: {
    requested_stale: number;
    charge_expired: number;
    payload_pending: number;
  };
};

export async function getAdminOverview(): Promise<AdminOverview> {
  return await adminFetch<AdminOverview>("/admin/overview");
}

export type AdminMeta = { mode: string };

export async function getAdminMeta(): Promise<AdminMeta> {
  return await adminFetch<AdminMeta>("/admin/meta");
}

export type AdminBotRow = {
  bot_id: string;
  bot_name?: string;
  created_at: string;
  last_seen_at?: string;
  revoked_at?: string;
  revoked: boolean;
};

export type AdminBotListResponse = {
  bots: AdminBotRow[];
  next_cursor?: string;
};

export async function listAdminBots(params: {
  q?: string;
  revoked?: string;
  cursor?: string;
  limit?: number;
}): Promise<AdminBotListResponse> {
  const url = new URL("/admin/bots", "http://localhost");
  if (params.q) url.searchParams.set("q", params.q);
  if (params.revoked) url.searchParams.set("revoked", params.revoked);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  if (params.limit) url.searchParams.set("limit", String(params.limit));
  return await adminFetch<AdminBotListResponse>(url.pathname + url.search);
}

export type AdminBotDetailResponse = {
  bot: {
    bot_id: string;
    bot_name?: string;
    signing_pubkey_ed25519: string;
    encryption_pubkey_x25519: string;
    signing_kid: string;
    encryption_kid: string;
    created_at: string;
    last_seen_at?: string;
    revoked: boolean;
    revoked_at?: string;
  };
  offers_total: number;
  jobs_total: number;
  payloads_pending: number;
};

export async function getAdminBot(botId: string): Promise<AdminBotDetailResponse> {
  return await adminFetch<AdminBotDetailResponse>(`/admin/bots/${botId}`);
}

export async function revokeAdminBot(botId: string, body: { reason: string; note?: string }) {
  return await adminFetch<{ audit_id: number; revoked_at: string; bot_id: string; revoked: boolean }>(
    `/admin/bots/${botId}/revoke`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }
  );
}

export type AdminOfferRow = {
  offer_id: string;
  seller_bot_id: string;
  seller_bot_name?: string;
  title: string;
  description: string;
  tags: string[];
  price_raw: string;
  turnaround_seconds: number;
  created_at: string;
  expires_at?: string;
  status: string;
  cancelled_at?: string;
  purchase_count: number;
};

export type AdminOfferListResponse = {
  offers: AdminOfferRow[];
  next_cursor?: string;
};

export async function listAdminOffers(params: {
  q?: string;
  seller_bot_id?: string;
  status?: string;
  tags?: string;
  cursor?: string;
  limit?: number;
}): Promise<AdminOfferListResponse> {
  const url = new URL("/admin/offers", "http://localhost");
  if (params.q) url.searchParams.set("q", params.q);
  if (params.seller_bot_id) url.searchParams.set("seller_bot_id", params.seller_bot_id);
  if (params.status) url.searchParams.set("status", params.status);
  if (params.tags) url.searchParams.set("tags", params.tags);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  if (params.limit) url.searchParams.set("limit", String(params.limit));
  return await adminFetch<AdminOfferListResponse>(url.pathname + url.search);
}

export type AdminOfferDetail = {
  offer_id: string;
  seller_bot_id: string;
  seller_bot_name?: string;
  title: string;
  description: string;
  tags: string[];
  price_raw: string;
  turnaround_seconds: number;
  created_at: string;
  expires_at?: string;
  status: string;
  cancelled_at?: string;
  request_schema_hint?: string;
  purchase_count: number;
};

export async function getAdminOffer(offerId: string): Promise<AdminOfferDetail> {
  return await adminFetch<AdminOfferDetail>(`/admin/offers/${offerId}`);
}

export async function moderateAdminOffer(
  offerId: string,
  action: "pause" | "resume" | "cancel",
  body: { reason: string; note?: string }
) {
  return await adminFetch<{ audit_id: number; offer: AdminOfferDetail }>(
    `/admin/offers/${offerId}/${action}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }
  );
}

export type AdminJobRow = {
  job_id: string;
  offer_id: string;
  buyer_bot_id: string;
  buyer_bot_name?: string;
  seller_bot_id: string;
  seller_bot_name?: string;
  status: string;
  price_raw: string;
  turnaround_seconds: number;
  created_at: string;
  job_expires_at: string;
  charge_id?: string;
  charge_address?: string;
  charge_amount_raw?: string;
  charge_expires_at?: string;
  paid_at?: string;
  delivered_at?: string;
  cancelled_at?: string;
  expired_at?: string;
};

export type AdminJobListResponse = { jobs: AdminJobRow[]; next_cursor?: string };

export async function listAdminJobs(params: {
  q?: string;
  offer_id?: string;
  buyer_bot_id?: string;
  seller_bot_id?: string;
  status?: string[];
  created_since?: string;
  cursor?: string;
  limit?: number;
}): Promise<AdminJobListResponse> {
  const url = new URL("/admin/jobs", "http://localhost");
  if (params.q) url.searchParams.set("q", params.q);
  if (params.offer_id) url.searchParams.set("offer_id", params.offer_id);
  if (params.buyer_bot_id) url.searchParams.set("buyer_bot_id", params.buyer_bot_id);
  if (params.seller_bot_id) url.searchParams.set("seller_bot_id", params.seller_bot_id);
  if (params.created_since) url.searchParams.set("created_since", params.created_since);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  if (params.limit) url.searchParams.set("limit", String(params.limit));
  if (params.status && params.status.length > 0) {
    for (const s of params.status) url.searchParams.append("status", s);
  }
  return await adminFetch<AdminJobListResponse>(url.pathname + url.search);
}

export type AdminJobDetailResponse = {
  job_id: string;
  offer_id: string;
  buyer_bot_id: string;
  buyer_bot_name?: string;
  seller_bot_id: string;
  seller_bot_name?: string;
  status: string;
  price_raw: string;
  turnaround_seconds: number;
  created_at: string;
  job_expires_at: string;
  request_payload_id: string;
  charge_id?: string;
  charge_address?: string;
  charge_amount_raw?: string;
  charge_expires_at?: string;
  charge_sig_ed25519?: string;
  paid_at?: string;
  delivered_at?: string;
  cancelled_at?: string;
  expired_at?: string;
  payment_verifier?: string;
  payment_block_hash?: string;
  payment_observed_at?: string;
  amount_raw_received?: string;
  payloads_pending: number;
  payloads_total: number;
};

export async function getAdminJob(jobId: string): Promise<AdminJobDetailResponse> {
  return await adminFetch<AdminJobDetailResponse>(`/admin/jobs/${jobId}`);
}

export async function moderateAdminJob(
  jobId: string,
  action: "cancel" | "expire",
  body: { reason: string; note?: string }
) {
  return await adminFetch<{ audit_id: number; job: AdminJobRow }>(
    `/admin/jobs/${jobId}/${action}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(body)
    }
  );
}

export type AdminPayloadRow = {
  payload_id: string;
  job_id: string;
  sender_bot_id: string;
  sender_bot_name?: string;
  recipient_bot_id: string;
  recipient_bot_name?: string;
  payload_kind: string;
  created_at: string;
  fetched_at?: string;
  ciphertext_b64_bytes: number;
};

export type AdminPayloadListResponse = { payloads: AdminPayloadRow[]; next_cursor?: string };

export async function listAdminPayloads(params: {
  status?: "unfetched" | "fetched" | "all";
  job_id?: string;
  recipient_bot_id?: string;
  sender_bot_id?: string;
  cursor?: string;
  limit?: number;
}): Promise<AdminPayloadListResponse> {
  const url = new URL("/admin/payloads", "http://localhost");
  if (params.status) url.searchParams.set("status", params.status);
  if (params.job_id) url.searchParams.set("job_id", params.job_id);
  if (params.recipient_bot_id) url.searchParams.set("recipient_bot_id", params.recipient_bot_id);
  if (params.sender_bot_id) url.searchParams.set("sender_bot_id", params.sender_bot_id);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  if (params.limit) url.searchParams.set("limit", String(params.limit));
  return await adminFetch<AdminPayloadListResponse>(url.pathname + url.search);
}

export type AdminAuditRow = {
  id: number;
  action: string;
  target_type: string;
  target_id: string;
  reason: string;
  note?: string;
  request_id?: string;
  token_fingerprint?: string;
  remote_addr?: string;
  user_agent?: string;
  before_json?: string;
  after_json?: string;
  created_at: string;
};

export type AdminAuditListResponse = { entries: AdminAuditRow[]; next_cursor?: string };

export async function listAdminAudit(params: {
  target_type?: string;
  target_id?: string;
  cursor?: string;
  limit?: number;
}): Promise<AdminAuditListResponse> {
  const url = new URL("/admin/audit", "http://localhost");
  if (params.target_type) url.searchParams.set("target_type", params.target_type);
  if (params.target_id) url.searchParams.set("target_id", params.target_id);
  if (params.cursor) url.searchParams.set("cursor", params.cursor);
  if (params.limit) url.searchParams.set("limit", String(params.limit));
  return await adminFetch<AdminAuditListResponse>(url.pathname + url.search);
}
