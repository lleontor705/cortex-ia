/**
 * Model Variants — OpenCode Plugin for Cortex-IA
 *
 * Fetches and caches per-model reasoning effort and parameter variants
 * on startup, saving an atomic cache to ~/.cortex/cache/model-variants.json.
 */

import type { Plugin } from "@opencode-ai/plugin"
import { mkdir, rename, writeFile } from "fs/promises"
import { homedir } from "os"
import path from "path"

export const ModelVariantsPlugin: Plugin = async (input) => {
  async function refreshVariantsCache() {
    try {
      const result = await input.client.provider.list()
      const data = (result as any).data ?? result
      const providerList: any[] = data?.all ?? data?.providers ?? (Array.isArray(data) ? data : [])

      const variants: Record<string, Record<string, string[]>> = {}
      for (const prov of providerList) {
        for (const [modelId, model] of Object.entries(prov.models ?? {})) {
          const m = model as any
          if (m.variants && Object.keys(m.variants).length > 0) {
            variants[prov.id] = variants[prov.id] || {}
            variants[prov.id][modelId] = Object.keys(m.variants).sort()
          }
        }
      }

      const cacheDir = path.join(homedir(), ".cortex", "cache")
      await mkdir(cacheDir, { recursive: true })

      const finalPath = path.join(cacheDir, "model-variants.json")
      const tmpPath = finalPath + ".tmp"
      await writeFile(tmpPath, JSON.stringify(variants, null, 2))
      await rename(tmpPath, finalPath)
      console.info("[model-variants] Cortex model variants cache updated successfully.")
    } catch (err) {
      console.error("[model-variants] Cache refresh failed:", err)
    }
  }

  // Fire and forget at initialization
  refreshVariantsCache().catch((err) => {
    console.error("[model-variants] Unexpected refresh error:", err)
  })

  return {}
}

export default ModelVariantsPlugin
