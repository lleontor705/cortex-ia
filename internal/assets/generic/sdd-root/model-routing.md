# Provider-Neutral Routing

OpenCode routing is optional. When a role has no explicit route, the renderer omits the `model` field and the role inherits the active model selected by OpenCode.

Routes use versioned semantic IDs such as `route/v1/architecture` and typed capability requirements, not implicit defaults.

Resolve an explicit route only from user/provider configuration or fresh qualified discovery, then emit its validated `provider/model`. Record provenance and degradation. Incomplete, stale, or unsatisfied explicit routes fail closed before emission; never synthesize a provider, model, or fallback.

Legacy profile values enter only through the profile decoder. Active output uses semantic route IDs and provenance.
