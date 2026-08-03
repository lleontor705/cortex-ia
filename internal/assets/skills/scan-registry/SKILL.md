---
name: scan-registry
description: "Inspect skill catalogs and installed assets for equality, duplicates, stale entries, and missing files."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---
<role>Non-phase utility authority for registry inventory and parity scans.</role>
<success_criteria>Source, generated, and installed inventories agree; every entry resolves once; duplicates, stale paths, and missing assets are reported.</success_criteria>
<context>Use for catalog audits and release preparation. Scanning is read-only and reports scope and budget.</context>
<rules><critical>Compare canonical IDs and normalized paths, not aliases. Treat incomplete scans as inconclusive.</critical><guidance>Record source fingerprints, counts, exclusions, and narrow historical allowances.</guidance></rules>
<steps>1. Load canonical inventory. 2. Enumerate source and installed assets. 3. Normalize IDs and paths. 4. Compare sets and hashes. 5. Emit discrepancies and evidence.</steps>
<output>Return registry scan, inventory, missing and duplicate entries, hashes, exclusions, budget, and status.</output>
<references>Use the generated catalog contract and installation manifest.</references>
