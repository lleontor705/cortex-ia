package diagram

// DiagramType enumerates the supported diagram families.
type DiagramType string

const (
	TypeArchitecture DiagramType = "architecture"
	TypeWorkflow     DiagramType = "workflow"
	TypeSequence     DiagramType = "sequence"
	TypeDataflow     DiagramType = "dataflow"
	TypeLifecycle    DiagramType = "lifecycle"
)

// QualityProfile defines validation strictness.
type QualityProfile string

const (
	QualityStandard QualityProfile = "standard"
	QualityShowcase QualityProfile = "showcase"
)

// Meta contains presentation and document configuration.
type Meta struct {
	Title              string         `json:"title"`
	Subtitle           string         `json:"subtitle,omitempty"`
	Locale             string         `json:"locale,omitempty"`
	VisualPreset       string         `json:"visual_preset,omitempty"`
	QualityProfile     QualityProfile `json:"quality_profile,omitempty"`
	EngineeringProfile string         `json:"engineering_profile,omitempty"`
	ViewBox            []float64      `json:"viewBox,omitempty"`
}

// Component represents one node in an architecture or dataflow diagram.
type Component struct {
	ID       string    `json:"id"`
	Type     string    `json:"type"`
	Label    string    `json:"label"`
	Sublabel string    `json:"sublabel,omitempty"`
	Tag      string    `json:"tag,omitempty"`
	Variant  string    `json:"variant,omitempty"`
	Brand    any       `json:"brand,omitempty"`
	Pos      []float64 `json:"pos,omitempty"`
	Role     string    `json:"role,omitempty"`
}

// Connection represents a directed edge between two components.
type Connection struct {
	ID       string `json:"id,omitempty"`
	From     string `json:"from"`
	To       string `json:"to"`
	Label    string `json:"label,omitempty"`
	Style    string `json:"style,omitempty"`
	Protocol string `json:"protocol,omitempty"`
	Variant  string `json:"variant,omitempty"`
}

// Boundary wraps a group of components inside a logical security or cloud domain.
type Boundary struct {
	ID    string   `json:"id"`
	Label string   `json:"label"`
	Type  string   `json:"type,omitempty"`
	Wraps []string `json:"wraps"`
}

// Card represents a conclusion or architectural detail card.
type Card struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Description string `json:"description,omitempty"`
}

// Step represents a node in a workflow or lifecycle diagram.
type Step struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Type        string `json:"type,omitempty"`
	Description string `json:"description,omitempty"`
}

// Message represents a sequence call between lifelines.
type Message struct {
	From  string `json:"from"`
	To    string `json:"to"`
	Label string `json:"label"`
	Async bool   `json:"async,omitempty"`
}

// DiagramSpecification is the unified JSON IR representation.
type DiagramSpecification struct {
	SchemaVersion int          `json:"schema_version"`
	DiagramType   DiagramType  `json:"diagram_type"`
	Meta          Meta         `json:"meta"`
	Components    []Component  `json:"components,omitempty"`
	Connections   []Connection `json:"connections,omitempty"`
	Boundaries    []Boundary   `json:"boundaries,omitempty"`
	Cards         []Card       `json:"cards,omitempty"`
	Steps         []Step       `json:"steps,omitempty"`
	Messages      []Message    `json:"messages,omitempty"`
	Participants  []Component  `json:"participants,omitempty"`
}
