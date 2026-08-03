# Provider-Neutral Routing

Resolve a semantic route at composition. A role without a primary and explicit fallback blocks before phase effects.

Routes use versioned identifiers such as `route/v1/architecture` and typed capability requirements. They are not provider or model names, quality tiers, or implicit defaults.

Resolve only from explicit user/provider configuration or fresh qualified discovery. Record source, freshness, capabilities, selected reference, and degradation reason. If no configured mapping satisfies the request, fail closed before emission or installation; never synthesize provider, model, or fallback.

Legacy values enter only through the versioned compatibility decoder. New profiles, manifests, receipts, prompts, generated assets, and docs contain semantic route IDs plus configuration provenance.
