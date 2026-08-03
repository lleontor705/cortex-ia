---
name: mutation-testing
description: "Run bounded mutation probes as a non-phase utility and report pass, fail, inconclusive, blocked, or degraded evidence."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---

# Mutation Testing Utility

This is a non-phase utility authority for execution-time mutation probes. It
does not edit production, install a product, or mutate a generated bundle.
Use `gomutants` only against an isolated copy and report deterministic,
inconclusive, blocked, pass, fail, or degraded outcomes with manifest evidence.
The utility is read-only with respect to the product and preserves the
repository's normal prepare/apply/rollback boundaries.
