/**
 * Pure state and validation primitives for the native OpenCode background-task
 * supervisor. This module deliberately performs no I/O and persists nothing.
 */

export const DISPATCH_OPEN = "<minion-dispatch>"
export const DISPATCH_CLOSE = "</minion-dispatch>"

export type MinionRole = "investigate" | "planner" | "implement" | "reviewer"
export type ExecutionClass = "reader" | "writer"
export type VerificationVerdict = "PASS" | "FAIL" | "BLOCKED" | "INCONCLUSIVE"
export type PhaseStatus = "success" | "partial" | "failed" | "blocked"
export type NativeSessionStatus =
  | "busy"
  | "idle"
  | "retry"
  | "cancelled"
  | "error"
  | "unknown"
export type EffectiveState =
  | "running"
  | "retrying"
  | "idle"
  | "cancelling"
  | "cancelled"
  | "error"
  | "unknown"

export interface DispatchBudget {
  max_turns: number
  max_retries: number
}

export interface MinionDispatchEnvelope {
  objective: string
  workflow: string
  task_id: string | null
  artifact_refs: string[]
  evidence_refs: string[]
  non_goals: string[]
  allowed_files: string[]
  allowed_effects: string[]
  required_skill: string | null
  acceptance_checks: string[]
  budget: DispatchBudget
  stop_conditions: string[]
  escalate_when: string[]
  role: MinionRole
  worktree_isolated?: boolean
}

export interface EnvelopeParseSuccess {
  ok: true
  envelope: MinionDispatchEnvelope
}

export interface EnvelopeParseFailure {
  ok: false
  errors: string[]
}

export type EnvelopeParseResult = EnvelopeParseSuccess | EnvelopeParseFailure

export interface MinionReceipt {
  receipt_version: string | null
  task_id: string | null
  phase_status: PhaseStatus | null
  task_status: string | null
  verification_verdict: VerificationVerdict
  evidence_refs: string[]
  valid: boolean
}

export interface BackgroundRecord {
  job_id: string
  session_id: string
  parent_session_id: string
  call_id: string | null
  role: MinionRole
  execution_class: ExecutionClass
  envelope: MinionDispatchEnvelope
  native_status: NativeSessionStatus
  cancel_requested: boolean
  cancel_acknowledged: boolean
  termination_confirmed: boolean
  verification_verdict: VerificationVerdict
  receipt: MinionReceipt | null
  error: string | null
  created_at: number
  updated_at: number
}

export type BackgroundEvent =
  | { type: "native_status"; status: NativeSessionStatus; at: number }
  | { type: "cancel_requested"; at: number }
  | { type: "cancel_acknowledged"; accepted: boolean; at: number }
  | { type: "termination_confirmed"; at: number }
  | { type: "receipt_observed"; receipt: MinionReceipt; at: number }
  | { type: "error"; message: string; at: number }

export interface ConcurrencyLimits {
  readers: number
  writers: number
}

export interface AdmissionDecision {
  allowed: boolean
  execution_class: ExecutionClass
  active: number
  limit: number
  reason: string | null
}

const ENVELOPE_KEYS = new Set([
  "objective",
  "workflow",
  "task_id",
  "artifact_refs",
  "evidence_refs",
  "non_goals",
  "allowed_files",
  "allowed_effects",
  "required_skill",
  "acceptance_checks",
  "budget",
  "stop_conditions",
  "escalate_when",
  "role",
  "worktree_isolated",
])

const BUDGET_KEYS = new Set(["max_turns", "max_retries"])
const ROLES = new Set<MinionRole>(["investigate", "planner", "implement", "reviewer"])
const PHASE_STATUSES = new Set<PhaseStatus>(["success", "partial", "failed", "blocked"])
const VERDICTS = new Set<VerificationVerdict>(["PASS", "FAIL", "BLOCKED", "INCONCLUSIVE"])
const TASK_STATUSES = new Set(["backlog", "ready", "in_progress", "in_review", "done", "blocked"])

const SECRET_KEY = /^(?:api[_-]?key|authorization|bearer|password|passwd|secret|access[_-]?token|refresh[_-]?token|claim[_-]?token|lease[_-]?token|private[_-]?key)$/i
const SECRET_TEXT = /(?:\bBearer\s+[A-Za-z0-9._~+/=-]{8,}|\bsk-[A-Za-z0-9_-]{8,}|\bgh[pousr]_[A-Za-z0-9_]{12,}|-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----)/i

function isObject(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value)
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((entry) => typeof entry === "string")
}

function hasSecret(value: unknown): boolean {
  if (typeof value === "string") return SECRET_TEXT.test(value)
  if (Array.isArray(value)) return value.some(hasSecret)
  if (!isObject(value)) return false
  return Object.entries(value).some(([key, child]) => SECRET_KEY.test(key) || hasSecret(child))
}

function requireNonEmptyString(
  value: unknown,
  field: string,
  errors: string[],
): value is string {
  if (typeof value !== "string" || value.trim().length === 0) {
    errors.push(`${field} must be a non-empty string`)
    return false
  }
  return true
}

function requireStringArray(value: unknown, field: string, errors: string[]): value is string[] {
  if (!isStringArray(value)) {
    errors.push(`${field} must be an array of strings`)
    return false
  }
  return true
}

/** Parse exactly one strict dispatch marker and reject authority material. */
export function parseMinionDispatch(input: string): EnvelopeParseResult {
  if (typeof input !== "string") {
    return { ok: false, errors: ["minion-dispatch input must be a string"] }
  }
  const errors: string[] = []
  const open = input.indexOf(DISPATCH_OPEN)
  const close = input.indexOf(DISPATCH_CLOSE)
  if (open < 0 || close < 0 || close < open) {
    return { ok: false, errors: ["missing or malformed minion-dispatch marker"] }
  }
  if (
    input.indexOf(DISPATCH_OPEN, open + DISPATCH_OPEN.length) >= 0 ||
    input.indexOf(DISPATCH_CLOSE, close + DISPATCH_CLOSE.length) >= 0
  ) {
    return { ok: false, errors: ["exactly one minion-dispatch marker is required"] }
  }

  const encoded = input.slice(open + DISPATCH_OPEN.length, close).trim()
  let value: unknown
  try {
    value = JSON.parse(encoded)
  } catch {
    return { ok: false, errors: ["minion-dispatch payload must be valid JSON"] }
  }
  if (!isObject(value)) return { ok: false, errors: ["minion-dispatch payload must be an object"] }

  if (!Object.prototype.hasOwnProperty.call(value, "task_id")) errors.push("task_id must be present (null is allowed)")
  for (const key of Object.keys(value)) {
    if (!ENVELOPE_KEYS.has(key)) errors.push(`unknown dispatch field: ${key}`)
  }
  if (hasSecret(value)) errors.push("dispatch envelope must not contain secrets or authority tokens")

  requireNonEmptyString(value.objective, "objective", errors)
  requireNonEmptyString(value.workflow, "workflow", errors)
  if (value.task_id !== null && !requireNonEmptyString(value.task_id, "task_id", errors)) {
    // Error added by the helper.
  }
  requireStringArray(value.artifact_refs, "artifact_refs", errors)
  requireStringArray(value.evidence_refs, "evidence_refs", errors)
  requireStringArray(value.non_goals, "non_goals", errors)
  requireStringArray(value.allowed_files, "allowed_files", errors)
  requireStringArray(value.allowed_effects, "allowed_effects", errors)
  if (value.required_skill !== null && !requireNonEmptyString(value.required_skill, "required_skill", errors)) {
    // Error added by the helper.
  }
  requireStringArray(value.acceptance_checks, "acceptance_checks", errors)
  requireStringArray(value.stop_conditions, "stop_conditions", errors)
  requireStringArray(value.escalate_when, "escalate_when", errors)

  if (!isObject(value.budget)) {
    errors.push("budget must be an object")
  } else {
    for (const key of Object.keys(value.budget)) {
      if (!BUDGET_KEYS.has(key)) errors.push(`unknown budget field: ${key}`)
    }
    if (!Number.isInteger(value.budget.max_turns) || Number(value.budget.max_turns) <= 0) {
      errors.push("budget.max_turns must be a positive integer")
    }
    if (!Number.isInteger(value.budget.max_retries) || Number(value.budget.max_retries) < 0) {
      errors.push("budget.max_retries must be a non-negative integer")
    }
  }

  if (typeof value.role !== "string" || !ROLES.has(value.role as MinionRole)) {
    errors.push("role must be one of investigate, planner, implement, reviewer")
  }
  if (value.worktree_isolated !== undefined && typeof value.worktree_isolated !== "boolean") {
    errors.push("worktree_isolated must be a boolean when present")
  }

  if (errors.length > 0) return { ok: false, errors }
  return { ok: true, envelope: value as unknown as MinionDispatchEnvelope }
}

export function classifyRole(role: MinionRole): ExecutionClass {
  return role === "implement" ? "writer" : "reader"
}

export function normalizeLimits(input: Partial<ConcurrencyLimits> = {}): ConcurrencyLimits {
  const readers = Number.isInteger(input.readers) && Number(input.readers) > 0 ? Number(input.readers) : 4
  const writers = Number.isInteger(input.writers) && Number(input.writers) > 0 ? Number(input.writers) : 1
  return { readers, writers }
}

export function effectiveState(record: BackgroundRecord): EffectiveState {
  if (record.native_status === "busy") return "running"
  if (record.native_status === "retry") return "retrying"
  if (record.native_status === "error") return "error"
  if (record.termination_confirmed) return "cancelled"
  if (record.cancel_requested || record.native_status === "cancelled") return "cancelling"
  if (record.native_status === "idle") return "idle"
  return "unknown"
}

export function isActive(record: BackgroundRecord): boolean {
  const state = effectiveState(record)
  return state === "running" || state === "retrying" || state === "cancelling"
}

export function evaluateAdmission(
  records: Iterable<BackgroundRecord>,
  role: MinionRole,
  limitsInput: Partial<ConcurrencyLimits> = {},
): AdmissionDecision {
  const executionClass = classifyRole(role)
  const limits = normalizeLimits(limitsInput)
  let active = 0
  for (const record of records) {
    if (record.execution_class === executionClass && isActive(record)) active += 1
  }
  const limit = executionClass === "reader" ? limits.readers : limits.writers
  const allowed = active < limit
  return {
    allowed,
    execution_class: executionClass,
    active,
    limit,
    reason: allowed ? null : `${executionClass} concurrency limit reached (${active}/${limit})`,
  }
}

export function reduceBackgroundRecord(
  record: BackgroundRecord,
  event: BackgroundEvent,
): BackgroundRecord {
  switch (event.type) {
    case "native_status":
      return {
        ...record,
        native_status: event.status,
        // A fresh busy event disproves a prior terminal observation.
        termination_confirmed: event.status === "busy" ? false : record.termination_confirmed,
        updated_at: event.at,
      }
    case "cancel_requested":
      return { ...record, cancel_requested: true, updated_at: event.at }
    case "cancel_acknowledged":
      return {
        ...record,
        cancel_requested: event.accepted,
        cancel_acknowledged: event.accepted,
        updated_at: event.at,
      }
    case "termination_confirmed":
      return record.native_status === "busy"
        ? { ...record, updated_at: event.at }
        : { ...record, termination_confirmed: true, updated_at: event.at }
    case "receipt_observed": {
      const trusted = event.receipt.valid && event.receipt.task_id === record.envelope.task_id
      const receipt = trusted
        ? event.receipt
        : { ...event.receipt, valid: false, verification_verdict: "INCONCLUSIVE" as const }
      return {
        ...record,
        receipt,
        verification_verdict: receipt.verification_verdict,
        updated_at: event.at,
      }
    }
    case "error":
      return {
        ...record,
        native_status: "error",
        error: sanitizeText(event.message),
        updated_at: event.at,
      }
  }
}

function jsonObjectCandidates(text: string): string[] {
  const candidates: string[] = []
  const fenced = /```(?:json)?\s*([\s\S]*?)```/gi
  for (const match of text.matchAll(fenced)) candidates.push(match[1].trim())

  for (let start = 0; start < text.length; start += 1) {
    if (text[start] !== "{") continue
    let depth = 0
    let quoted = false
    let escaped = false
    for (let cursor = start; cursor < text.length; cursor += 1) {
      const char = text[cursor]
      if (quoted) {
        if (escaped) escaped = false
        else if (char === "\\") escaped = true
        else if (char === '"') quoted = false
        continue
      }
      if (char === '"') quoted = true
      else if (char === "{") depth += 1
      else if (char === "}") {
        depth -= 1
        if (depth === 0) {
          candidates.push(text.slice(start, cursor + 1))
          start = cursor
          break
        }
      }
    }
  }
  return candidates
}

/** Parse the last receipt-shaped JSON object. Missing/invalid verdicts stay INCONCLUSIVE. */
export function parseMinionReceipt(text: string): MinionReceipt | null {
  const candidates = jsonObjectCandidates(text)
  for (let index = candidates.length - 1; index >= 0; index -= 1) {
    let value: unknown
    try {
      value = JSON.parse(candidates[index])
    } catch {
      continue
    }
    if (!isObject(value)) continue
    const receiptShaped =
      "verification_verdict" in value ||
      "phase_status" in value ||
      "task_status" in value ||
      "receipt_version" in value
    if (!receiptShaped) continue

    const rawVerdict = value.verification_verdict
    const recognizedVerdict =
      typeof rawVerdict === "string" && VERDICTS.has(rawVerdict as VerificationVerdict)
    const verdict = recognizedVerdict
      ? (rawVerdict as VerificationVerdict)
      : "INCONCLUSIVE"
    const phase =
      typeof value.phase_status === "string" && PHASE_STATUSES.has(value.phase_status as PhaseStatus)
        ? (value.phase_status as PhaseStatus)
        : null
    const evidence = isStringArray(value.evidence_refs)
      ? value.evidence_refs.map(sanitizeText)
      : []
    const taskIdPresent = Object.prototype.hasOwnProperty.call(value, "task_id")
    const taskId = taskIdPresent && (value.task_id === null || typeof value.task_id === "string")
      ? value.task_id
      : null
    const taskStatus = typeof value.task_status === "string" && TASK_STATUSES.has(value.task_status)
      ? value.task_status
      : null
    const receiptVersion = typeof value.receipt_version === "string" ? sanitizeText(value.receipt_version) : null

    return {
      receipt_version: receiptVersion,
      task_id: taskId,
      phase_status: phase,
      task_status: taskStatus,
      verification_verdict: verdict,
      evidence_refs: evidence,
      valid: recognizedVerdict && taskIdPresent && phase !== null && taskStatus !== null,
    }
  }
  return null
}

export function sanitizeText(text: string): string {
  return text
    .replace(/\bBearer\s+[A-Za-z0-9._~+/=-]{8,}/gi, "Bearer [REDACTED]")
    .replace(/\bsk-[A-Za-z0-9_-]{8,}/gi, "[REDACTED]")
    .replace(/\bgh[pousr]_[A-Za-z0-9_]{12,}/gi, "[REDACTED]")
    .replace(/-----BEGIN (?:RSA |EC |OPENSSH )?PRIVATE KEY-----[\s\S]*?-----END (?:RSA |EC |OPENSSH )?PRIVATE KEY-----/gi, "[REDACTED PRIVATE KEY]")
}

export function sanitizeValue(value: unknown): unknown {
  if (typeof value === "string") return sanitizeText(value)
  if (Array.isArray(value)) return value.map(sanitizeValue)
  if (!isObject(value)) return value
  const output: Record<string, unknown> = {}
  for (const [key, child] of Object.entries(value)) {
    output[key] = SECRET_KEY.test(key) ? "[REDACTED]" : sanitizeValue(child)
  }
  return output
}

/** Bounded, prompt-free state for compaction context. */
export function formatCompactionSummary(
  records: Iterable<BackgroundRecord>,
  maxChars = 4000,
): string {
  const safeMax = Number.isInteger(maxChars) && maxChars >= 128 ? maxChars : 4000
  const rows = [...records]
    .sort((left, right) => right.updated_at - left.updated_at)
    .map((record) => {
      const task = record.envelope.task_id ?? "direct"
      return [
        `job=${sanitizeText(record.job_id)}`,
        `session=${sanitizeText(record.session_id)}`,
        `task=${sanitizeText(task)}`,
        `role=${record.role}`,
        `class=${record.execution_class}`,
        `state=${effectiveState(record)}`,
        `verdict=${record.verification_verdict}`,
      ].join(" ")
    })
  const header = "Background supervisor (advisory; ForgeSpec remains authoritative):"
  if (rows.length === 0) return `${header}\n- no observed jobs`

  let output = header
  for (const row of rows) {
    const candidate = `${output}\n- ${row}`
    if (candidate.length > safeMax) break
    output = candidate
  }
  if (output.length === header.length) return `${header}\n- summary omitted by size bound`
  if (output.length < header.length + rows.join("").length) {
    const suffix = "\n- additional jobs omitted"
    if (output.length + suffix.length <= safeMax) output += suffix
  }
  return sanitizeText(output).slice(0, safeMax)
}
