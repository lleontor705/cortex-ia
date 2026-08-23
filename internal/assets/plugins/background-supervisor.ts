import type { Plugin } from "@opencode-ai/plugin"
import { tool } from "@opencode-ai/plugin"
import {
  classifyRole,
  effectiveState,
  evaluateAdmission,
  formatCompactionSummary,
  isActive,
  normalizeLimits,
  parseMinionDispatch,
  parseMinionReceipt,
  reduceBackgroundRecord,
  sanitizeText,
  sanitizeValue,
  type BackgroundRecord,
  type MinionDispatchEnvelope,
  type MinionRole,
  type NativeSessionStatus,
} from "./background-supervisor-core"

const FLAG = "OPENCODE_EXPERIMENTAL_BACKGROUND_SUBAGENTS"
const VERSION = "1.0.0"
const MAX_RECOVERY_DEPTH = 5
const MAX_RECORDS = 200
const PENDING_TTL_MS = 5 * 60 * 1000
export function prunePendingDispatches<T extends { createdAt: number }>(items: Map<string, T>, now = Date.now(), ttl = PENDING_TTL_MS) {
  let removed = 0
  for (const [id, item] of items) if (now - item.createdAt >= ttl) { items.delete(id); removed += 1 }
  return removed
}

const MAX_RECOVERY_SESSIONS = 100
type Capability = "unknown" | "supported" | "unsupported"
type Pending = { callID: string; parentID: string; envelope: MinionDispatchEnvelope; createdAt: number }
type NativeSession = { id: string; parentID?: string; title?: string; time?: { created?: number; updated?: number } }

function flagEnabled() {
  return process.env[FLAG]?.trim().toLowerCase() === "true"
}

function positiveEnv(name: string, fallback: number) {
  const value = process.env[name]
  if (!value || !/^\d+$/.test(value)) return fallback
  const parsed = Number(value)
  return Number.isSafeInteger(parsed) && parsed > 0 ? parsed : fallback
}

function safeError(error: unknown) {
  if (error instanceof Error) return sanitizeText(error.message).slice(0, 1000)
  try { return sanitizeText(JSON.stringify(sanitizeValue(error))).slice(0, 1000) } catch { return "unknown error" }
}

function data<T>(response: unknown, operation: string): T {
  if (!response || typeof response !== "object") throw new Error(`${operation} returned an invalid response`)
  const result = response as { data?: T; error?: unknown }
  if (result.error !== undefined) throw new Error(`${operation} failed: ${safeError(result.error)}`)
  if (!("data" in result)) throw new Error(`${operation} returned no data`)
  return result.data as T
}

function json(value: unknown) {
  return JSON.stringify(sanitizeValue(value), null, 2)
}

function nativeStatus(value: unknown): NativeSessionStatus {
  const type = value && typeof value === "object" ? (value as { type?: string }).type : undefined
  return type === "busy" || type === "idle" || type === "retry" ? type : "unknown"
}

function roleFromTitle(title?: string): MinionRole | null {
  const match = title?.match(/@(?:agent-)?(investigate|planner|implement|reviewer)\b/i)
  return (match?.[1]?.toLowerCase() as MinionRole | undefined) ?? null
}

function makeRecord(input: {
  jobID: string; sessionID: string; parentID: string; envelope: MinionDispatchEnvelope
  callID?: string | null; status?: NativeSessionStatus; createdAt?: number; updatedAt?: number
}): BackgroundRecord {
  const now = Date.now()
  return {
    job_id: input.jobID, session_id: input.sessionID, parent_session_id: input.parentID,
    call_id: input.callID ?? null, role: input.envelope.role,
    execution_class: classifyRole(input.envelope.role), envelope: input.envelope,
    native_status: input.status ?? "busy", cancel_requested: false, cancel_acknowledged: false,
    termination_confirmed: false, verification_verdict: "INCONCLUSIVE",
    receipt: null, error: null, created_at: input.createdAt ?? now, updated_at: input.updatedAt ?? now,
  }
}

function publicRecord(record: BackgroundRecord) {
  return {
    job_id: record.job_id, session_id: record.session_id, parent_session_id: record.parent_session_id,
    call_id: record.call_id, task_id: record.envelope.task_id, workflow: record.envelope.workflow,
    role: record.role, execution_class: record.execution_class, native_status: record.native_status,
    effective_state: effectiveState(record), cancel_requested: record.cancel_requested,
    cancel_acknowledged: record.cancel_acknowledged,
    verification_verdict: record.verification_verdict, receipt: record.receipt, error: record.error,
    created_at: record.created_at, updated_at: record.updated_at,
  }
}

function textParts(messages: unknown, role?: "assistant" | "user") {
  if (!Array.isArray(messages)) return ""
  const rows: string[] = []
  for (const message of messages) {
    const item = message as { info?: { role?: string }; parts?: Array<{ type?: string; text?: string }> }
    if (role && item.info?.role !== role) continue
    for (const part of item.parts ?? []) {
      if (part.type === "text" && typeof part.text === "string") {
        rows.push(role ? part.text : `[${item.info?.role ?? "message"}] ${sanitizeText(part.text)}`)
      }
    }
  }
  return rows.join("\n")
}

export const BackgroundSupervisorPlugin: Plugin = async ({ client, directory }) => {
  const records = new Map<string, BackgroundRecord>()
  const pending = new Map<string, Pending>()
  const children = new Set<string>()
  const sessionAgents = new Map<string, string>()
  let capability: Capability = "unknown"
  const limits = () => normalizeLimits({
    readers: positiveEnv("OPENCODE_BG_MAX_READERS", 4),
    writers: positiveEnv("OPENCODE_BG_MAX_WRITERS", 1),
  })
  const prunePending = () => prunePendingDispatches(pending)
  const pruneRecords = () => {
    if (records.size <= MAX_RECORDS) return 0
    const terminal = [...records.entries()]
      .filter(([, record]) => !isActive(record))
      .sort((left, right) => left[1].updated_at - right[1].updated_at)
    let removed = 0
    for (const [id] of terminal) {
      if (records.size <= MAX_RECORDS) break
      records.delete(id)
      removed += 1
    }
    return removed
  }
  const byTarget = (target: string) => records.get(target) ?? [...records.values()].find((r) => r.session_id === target)
  const pendingRecords = () => {
    prunePending()
    return [...pending.values()].map((p) => makeRecord({
      jobID: `pending:${p.callID}`, sessionID: p.parentID, parentID: p.parentID,
      callID: p.callID, envelope: p.envelope,
    }))
  }

  function setStatus(sessionID: string, status: NativeSessionStatus) {
    for (const [id, record] of records) if (record.session_id === sessionID) {
      records.set(id, reduceBackgroundRecord(record, { type: "native_status", status, at: Date.now() }))
    }
  }

  function observeReceipt(sessionID: string, text: string) {
    const receipt = parseMinionReceipt(text)
    if (!receipt) return
    for (const [id, record] of records) if (record.session_id === sessionID) {
      records.set(id, reduceBackgroundRecord(record, { type: "receipt_observed", receipt, at: Date.now() }))
    }
  }

  async function reconcile(selected = [...records.values()]) {
    if (!selected.length) return selected
    const statuses = data<Record<string, unknown>>(
      await client.session.status({ query: { directory } }), "session.status",
    )
    for (const record of selected) {
      const status = Object.hasOwn(statuses, record.session_id) ? nativeStatus(statuses[record.session_id]) : "idle"
      let next = reduceBackgroundRecord(record, { type: "native_status", status, at: Date.now() })
      if (next.cancel_requested && next.cancel_acknowledged && status === "idle") {
        next = reduceBackgroundRecord(next, { type: "termination_confirmed", at: Date.now() })
      }
      records.set(record.job_id, next)
    }
    await Promise.all(selected.filter((r) => records.get(r.job_id)?.native_status !== "busy").map(async (record) => {
      try {
        const messages = data<unknown[]>(await client.session.messages({
          path: { id: record.session_id }, query: { directory, limit: 50 },
        }), "session.messages")
        observeReceipt(record.session_id, textParts(messages, "assistant"))
      } catch (error) {
        const current = records.get(record.job_id)
        if (current) records.set(record.job_id, reduceBackgroundRecord(current, {
          type: "error", message: safeError(error), at: Date.now(),
        }))
      }
    }))
    pruneRecords()
    return selected.map((r) => records.get(r.job_id) ?? r)
  }

  async function recover(rootID: string, requestedDepth: number) {
    const depth = Math.max(1, Math.min(MAX_RECOVERY_DEPTH, requestedDepth))
    const queue = [{ id: rootID, level: 0 }]
    const visited = new Set([rootID])
    let examined = 0
    let adopted = 0
    while (queue.length && examined < MAX_RECOVERY_SESSIONS) {
      const parent = queue.shift()!
      if (parent.level >= depth) continue
      const found = data<NativeSession[]>(await client.session.children({
        path: { id: parent.id }, query: { directory },
      }), "session.children")
      for (const child of found) {
        if (!child?.id || visited.has(child.id)) continue
        visited.add(child.id); examined += 1
        const existing = [...records.values()].some((record) => record.session_id === child.id)
        if (existing) {
          if (examined < MAX_RECOVERY_SESSIONS) queue.push({ id: child.id, level: parent.level + 1 })
          continue
        }
        let messages: unknown[]
        try {
          messages = data<unknown[]>(await client.session.messages({
            path: { id: child.id }, query: { directory, limit: 10 },
          }), "session.messages")
        } catch {
          continue
        }
        const parsed = parseMinionDispatch(textParts(messages, "user"))
        const role = roleFromTitle(child.title)
        if (!parsed.ok || !role || parsed.envelope.role !== role) continue
        children.add(child.id)
        records.set(child.id, makeRecord({
          jobID: child.id, sessionID: child.id, parentID: child.parentID ?? parent.id,
          envelope: parsed.envelope, status: "unknown",
          createdAt: child.time?.created, updatedAt: child.time?.updated,
        }))
        adopted += 1
        if (examined < MAX_RECOVERY_SESSIONS) queue.push({ id: child.id, level: parent.level + 1 })
      }
    }
    await reconcile()
    pruneRecords()
    return { examined, adopted, depth, truncated: examined >= MAX_RECOVERY_SESSIONS }
  }

  async function isNested(sessionID: string) {
    if (children.has(sessionID)) return true
    const agent = sessionAgents.get(sessionID)
    if (agent && agent !== "orchestrator") return true
    try {
      const session = data<NativeSession>(await client.session.get({
        path: { id: sessionID }, query: { directory },
      }), "session.get")
      if (session.parentID) { children.add(sessionID); return true }
    } catch { return Boolean(agent && agent !== "orchestrator") }
    return false
  }

  return {
    tool: {
      background_doctor: tool({
        description: "Inspect native background capability, limits, and advisory supervisor state.", args: {},
        async execute() {
          prunePending()
          pruneRecords()
          return json({
            plugin: "background-supervisor", version: VERSION, api: "legacy",
            experimental_flag: { name: FLAG, enabled: flagEnabled() },
            native_task_background_parameter: capability, limits: limits(),
            observed_jobs: records.size, pending_dispatches: pending.size,
            authority: "ForgeSpec remains authoritative; supervisor state is advisory and reconstructible",
          })
        },
      }),
      background_status: tool({
        description: "Reconcile native background sessions and return sanitized advisory status.",
        args: { job_id: tool.schema.string().optional() },
        async execute(args) { try {
          const selected = args.job_id ? [byTarget(args.job_id)].filter(Boolean) as BackgroundRecord[] : [...records.values()]
          if (args.job_id && !selected.length) throw new Error("background job is not owned by this supervisor")
          return json({ authority: "advisory", jobs: (await reconcile(selected)).map(publicRecord) })
        } catch (error) { return json({ ok: false, error: safeError(error) }) } },
      }),
      background_tail: tool({
        description: "Read a bounded sanitized child-session tail without interrupting it.",
        args: { job_id: tool.schema.string(), limit: tool.schema.number().int().min(1).max(50).optional() },
        async execute(args) { try {
          const record = byTarget(args.job_id)
          if (!record) throw new Error("background job is not owned by this supervisor")
          const messages = data<unknown[]>(await client.session.messages({
            path: { id: record.session_id }, query: { directory, limit: args.limit ?? 12 },
          }), "session.messages")
          observeReceipt(record.session_id, textParts(messages, "assistant"))
          return json({ job_id: record.job_id, session_id: record.session_id, tail: textParts(messages).slice(-12000) })
        } catch (error) { return json({ ok: false, error: safeError(error) }) } },
      }),
      background_cancel: tool({
        description: "Abort one owned child session without deleting it or its evidence.",
        args: { job_id: tool.schema.string() },
        async execute(args) { try {
          const record = byTarget(args.job_id)
          if (!record) throw new Error("background job is not owned by this supervisor")
          const accepted = data<boolean>(await client.session.abort({
            path: { id: record.session_id }, query: { directory },
          }), "session.abort")
          const requested = reduceBackgroundRecord(record, { type: "cancel_requested", at: Date.now() })
          const acknowledged = reduceBackgroundRecord(requested, { type: "cancel_acknowledged", accepted, at: Date.now() })
          records.set(record.job_id, acknowledged)
          const [current] = await reconcile([acknowledged])
          return json({ accepted, job: publicRecord(current ?? acknowledged) })
        } catch (error) { return json({ ok: false, error: safeError(error) }) } },
      }),
      background_recover: tool({
        description: "Rebuild advisory state from native child-session relationships.",
        args: {
          depth: tool.schema.number().int().min(1).max(MAX_RECOVERY_DEPTH).optional(),
        },
        async execute(args, context) { try {
          const result = await recover(context.sessionID, args.depth ?? 3)
          return json({ ...result, root_session_id: context.sessionID, jobs: [...records.values()].map(publicRecord) })
        } catch (error) { return json({ ok: false, error: safeError(error) }) } },
      }),
    },
    async "chat.message"(input) { if (input.agent) sessionAgents.set(input.sessionID, input.agent) },
    async "tool.definition"(input, output) {
      if (input.toolID.toLowerCase() !== "task") return
      const parameters = output.parameters as { properties?: Record<string, unknown>; shape?: Record<string, unknown> }
      capability = parameters?.properties?.background || parameters?.shape?.background ? "supported" : "unsupported"
      output.description += " For supervised async delegation set background=true and include one strict <minion-dispatch> JSON envelope."
    },
    async "tool.execute.before"(input, output) {
      if (input.tool.toLowerCase() !== "task" || output.args?.background !== true) return
      if (!flagEnabled()) throw new Error(`${FLAG}=true is required for native background subagents`)
      if (await isNested(input.sessionID)) throw new Error("nested delegation denied: only orchestrator may launch background subagents")
      const parsed = parseMinionDispatch(output.args?.prompt)
      if (!parsed.ok) throw new Error(`invalid minion dispatch: ${sanitizeText(parsed.errors.join("; "))}`)
      if (output.args?.subagent_type !== parsed.envelope.role) throw new Error("dispatch role must match task.subagent_type")
      if (parsed.envelope.role === "implement" && !parsed.envelope.allowed_files.length) {
        throw new Error("implement dispatch requires at least one allowed_files entry")
      }
      const decision = evaluateAdmission(
        [...records.values(), ...pendingRecords()], parsed.envelope.role, limits(),
      )
      if (!decision.allowed) throw new Error(decision.reason ?? "background concurrency limit reached")
      pending.set(input.callID, {
        callID: input.callID, parentID: input.sessionID, envelope: parsed.envelope, createdAt: Date.now(),
      })
    },
    async "tool.execute.after"(input, output) {
      if (input.tool.toLowerCase() !== "task" || input.args?.background !== true) return
      const item = pending.get(input.callID); pending.delete(input.callID)
      const metadata = (output.metadata ?? {}) as Record<string, unknown>
      const sessionID = typeof metadata.sessionId === "string" ? metadata.sessionId :
        typeof metadata.sessionID === "string" ? metadata.sessionID : null
      const jobID = typeof metadata.jobId === "string" ? metadata.jobId : sessionID
      const parentID = typeof metadata.parentSessionId === "string" ? metadata.parentSessionId : input.sessionID
      if (!item || !sessionID || !jobID) return
      children.add(sessionID)
      let record = makeRecord({
        jobID, sessionID, parentID, callID: input.callID, envelope: item.envelope, createdAt: item.createdAt,
      })
      const receipt = parseMinionReceipt(output.output)
      if (receipt) record = reduceBackgroundRecord(record, { type: "receipt_observed", receipt, at: Date.now() })
      records.set(jobID, record)
      pruneRecords()
    },
    async event({ event }) {
      const current = event as unknown as { type?: string; properties?: Record<string, any> }
      const p = current.properties ?? {}
      if (current.type === "session.status" && typeof p.sessionID === "string") setStatus(p.sessionID, nativeStatus(p.status))
      else if (current.type === "session.idle" && typeof p.sessionID === "string") setStatus(p.sessionID, "idle")
      else if ((current.type === "session.created" || current.type === "session.updated") && p.info?.parentID) children.add(p.info.id)
      else if (current.type === "session.deleted" && typeof p.info?.id === "string") {
        setStatus(p.info.id, "cancelled")
        for (const [id, record] of records) if (record.session_id === p.info.id) {
          records.set(id, reduceBackgroundRecord(record, { type: "termination_confirmed", at: Date.now() }))
        }
      } else if (current.type === "session.error" && typeof p.sessionID === "string") {
        for (const [id, record] of records) if (record.session_id === p.sessionID) {
          records.set(id, reduceBackgroundRecord(record, { type: "error", message: safeError(p.error), at: Date.now() }))
        }
      } else if (current.type === "message.part.updated" && p.part?.type === "text") {
        observeReceipt(p.part.sessionID, p.part.text)
      }
    },
    async "experimental.chat.system.transform"(input, output) {
      const agent = input.sessionID ? sessionAgents.get(input.sessionID) : undefined
      if (!agent || agent === "orchestrator") output.system.push(
        "Only orchestrator may use task(background=true). Include one strict <minion-dispatch>, respect admission, and use background_status/recover instead of polling. ForgeSpec remains authoritative.",
      )
    },
    async "experimental.session.compacting"(input, output) {
      output.context.push(formatCompactionSummary([...records.values()].filter(
        (r) => r.parent_session_id === input.sessionID || r.session_id === input.sessionID,
      )))
    },
  }
}

export default BackgroundSupervisorPlugin
