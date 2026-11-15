package modelmgr

// NanProvider is the provider id whose models carry static capability metadata.
const NanProvider = "nan"

// Effort posture tokens. The posture is what the API does with a requested
// reasoning level, independent of whether the level is enumerable: an
// adjustable model honors the value, while adaptive and accepted models
// tolerate it without changing behavior, so an authored variant is
// informational only.
const (
	// NanEffortAdjustable models honor the requested reasoning level.
	NanEffortAdjustable = "adjustable"

	// NanEffortAdaptive models accept any level and ignore it.
	NanEffortAdaptive = "adaptive"

	// NanEffortAccepted models accept documented levels and ignore them.
	NanEffortAccepted = "accepted-not-adjustable"
)

// Tier tokens order the picker: premium and standard are selectable chat
// models, legacy is deprioritized behind its preferred replacement, and
// utility covers non-chat models that are never picker candidates.
const (
	NanTierPremium  = "premium"
	NanTierStandard = "standard"
	NanTierLegacy   = "legacy"
	NanTierUtility  = "utility"
)

// NanModelMeta is the static, receiving-side capability record for one nan
// model. The daemon publishes model ids and variants but never plan quotas or
// context limits, so these facts are pinned here instead of derived from an
// acquisition tier. A monthly quota of zero means unknown, never unlimited:
// consumers must suppress a quota percentage rather than divide by zero.
type NanModelMeta struct {
	Model              string   `json:"model"`
	Tier               string   `json:"tier"`
	ContextTokens      int64    `json:"contextTokens"`
	MaxOutputTokens    int64    `json:"maxOutputTokens,omitempty"`
	MonthlyQuotaTokens int64    `json:"monthlyQuotaTokens"`
	Rolling4hRefTokens int64    `json:"rolling4hRefTokens,omitempty"`
	EffortVocabulary   []string `json:"effortVocabulary"`
	EffortAdjustable   bool     `json:"effortAdjustable"`
	EffortMode         string   `json:"effortMode,omitempty"`
	Modalities         []string `json:"modalities"`
	PickerEligible     bool     `json:"pickerEligible"`
	PreferredOver      string   `json:"preferredOver,omitempty"`
}

// nanCatalog is the single static source of truth for nan model capabilities,
// ratified from the provider capability matrix. Slices are non-nil so receipts
// always render a stable array instead of null, and rows lead with the picker
// preference order so a filtered receipt is deterministic without sorting.
var nanCatalog = []NanModelMeta{
	{
		Model:              "glm5.3",
		Tier:               NanTierPremium,
		ContextTokens:      1_000_000,
		MonthlyQuotaTokens: 3_000_000_000,
		Rolling4hRefTokens: 400_000_000,
		EffortVocabulary:   []string{"low", "medium", "high", "max"},
		EffortAdjustable:   true,
		EffortMode:         NanEffortAdjustable,
		Modalities:         []string{"text"},
		PickerEligible:     true,
	},
	{
		Model:              "glm5.3-flash",
		Tier:               NanTierStandard,
		ContextTokens:      1_000_000,
		MonthlyQuotaTokens: 2_000_000_000,
		EffortVocabulary:   []string{"low", "medium", "high", "max"},
		EffortAdjustable:   true,
		EffortMode:         NanEffortAdjustable,
		Modalities:         []string{"text", "image"},
		PickerEligible:     true,
	},
	{
		Model:              "deepseek-v4-flash",
		Tier:               NanTierStandard,
		ContextTokens:      1_000_000,
		MonthlyQuotaTokens: 3_000_000_000,
		EffortVocabulary:   []string{},
		EffortAdjustable:   false,
		EffortMode:         NanEffortAdaptive,
		Modalities:         []string{"text", "image"},
		PickerEligible:     true,
		PreferredOver:      "qwen3.6",
	},
	{
		Model:              "qwen3.8-flash",
		Tier:               NanTierStandard,
		ContextTokens:      262_144,
		MaxOutputTokens:    131_072,
		MonthlyQuotaTokens: 500_000_000,
		EffortVocabulary:   []string{},
		EffortAdjustable:   false,
		EffortMode:         NanEffortAccepted,
		Modalities:         []string{"text", "image"},
		PickerEligible:     true,
	},
	{
		Model:              "mimo-v2.5",
		Tier:               NanTierStandard,
		ContextTokens:      1_000_000,
		MaxOutputTokens:    131_072,
		MonthlyQuotaTokens: 1_000_000_000,
		EffortVocabulary:   []string{},
		EffortAdjustable:   false,
		EffortMode:         NanEffortAccepted,
		Modalities:         []string{"text", "image", "audio"},
		PickerEligible:     true,
	},
	{
		Model:            "gemma4",
		Tier:             NanTierStandard,
		ContextTokens:    262_144,
		EffortVocabulary: []string{"none", "minimal", "low", "medium", "high", "max"},
		EffortAdjustable: true,
		EffortMode:       NanEffortAdjustable,
		Modalities:       []string{"text", "image"},
		PickerEligible:   true,
	},
	{
		Model:            "qwen3.6",
		Tier:             NanTierLegacy,
		ContextTokens:    262_144,
		EffortVocabulary: []string{"none", "minimal", "low", "medium", "high", "max"},
		EffortAdjustable: true,
		EffortMode:       NanEffortAdjustable,
		Modalities:       []string{"text", "image"},
		PickerEligible:   true,
	},
	{
		Model:            "qwen3-embedding",
		Tier:             NanTierUtility,
		EffortVocabulary: []string{},
		Modalities:       []string{"text"},
	},
	{
		Model:            "rerank",
		Tier:             NanTierUtility,
		EffortVocabulary: []string{},
		Modalities:       []string{"text"},
	},
	{
		Model:            "kokoro",
		Tier:             NanTierUtility,
		EffortVocabulary: []string{},
		Modalities:       []string{"audio"},
	},
	{
		Model:            "whisper",
		Tier:             NanTierUtility,
		EffortVocabulary: []string{},
		Modalities:       []string{"audio"},
	},
	{
		Model:            "flux-2-klein",
		Tier:             NanTierUtility,
		EffortVocabulary: []string{},
		Modalities:       []string{"image"},
	},
}

// NanModelMetaFor returns the static record for a nan model id. The result is a
// defensive copy, so callers can never mutate the shared table.
func NanModelMetaFor(model string) (NanModelMeta, bool) {
	for _, meta := range nanCatalog {
		if meta.Model == model {
			return cloneNanModelMeta(meta), true
		}
	}
	return NanModelMeta{}, false
}

// attachNanMeta decorates nan entries with their static metadata in a fresh
// slice. Non-nan entries are copied unchanged and carry no meta, so their
// receipt shape stays identical. A picker-eligible, effort-adjustable nan entry
// whose acquisition source yielded no variants inherits the static vocabulary,
// because the CLI publishes model ids but never enumerates effort levels.
func attachNanMeta(entries []CatalogEntry) []CatalogEntry {
	decorated := make([]CatalogEntry, len(entries))
	copy(decorated, entries)
	for i := range decorated {
		if decorated[i].Provider != NanProvider {
			continue
		}
		meta, ok := NanModelMetaFor(decorated[i].Model)
		if !ok {
			continue
		}
		record := meta
		if len(decorated[i].Variants) == 0 && record.PickerEligible && record.EffortAdjustable && len(record.EffortVocabulary) > 0 {
			decorated[i].Variants = append(make([]string, 0, len(record.EffortVocabulary)), record.EffortVocabulary...)
		}
		decorated[i].Meta = &record
	}
	return decorated
}

func cloneNanModelMeta(meta NanModelMeta) NanModelMeta {
	clone := meta
	clone.EffortVocabulary = append(make([]string, 0, len(meta.EffortVocabulary)), meta.EffortVocabulary...)
	clone.Modalities = append(make([]string, 0, len(meta.Modalities)), meta.Modalities...)
	return clone
}
