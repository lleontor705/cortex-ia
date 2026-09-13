package diagram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// RenderResult records details of the rendered HTML artifact.
type RenderResult struct {
	OutputPath  string `json:"output_path"`
	DiagramType string `json:"diagram_type"`
	Title       string `json:"title"`
	ByteCount   int    `json:"byte_count"`
	EngineUsed  string `json:"engine_used"`
}

// RenderOptions controls rendering.
type RenderOptions struct {
	DiagramType DiagramType
	Quality     QualityProfile
	Timeout     time.Duration
}

// RenderFile reads the specification and renders it to the target HTML file.
func RenderFile(ctx context.Context, inputJsonPath, outputHtmlPath string, opts RenderOptions) (*RenderResult, error) {
	data, err := os.ReadFile(inputJsonPath)
	if err != nil {
		return nil, fmt.Errorf("read input specification %s: %w", inputJsonPath, err)
	}
	return Render(ctx, data, outputHtmlPath, opts)
}

// Render processes diagram JSON bytes and writes a self-contained interactive HTML file.
func Render(ctx context.Context, specData []byte, outputHtmlPath string, opts RenderOptions) (*RenderResult, error) {
	var spec DiagramSpecification
	if err := json.Unmarshal(specData, &spec); err != nil {
		return nil, fmt.Errorf("invalid specification JSON: %w", err)
	}

	diagType := opts.DiagramType
	if diagType == "" {
		diagType = spec.DiagramType
	}
	if diagType == "" {
		diagType = TypeArchitecture
	}

	outClean := filepath.Clean(outputHtmlPath)
	if err := os.MkdirAll(filepath.Dir(outClean), 0755); err != nil {
		return nil, fmt.Errorf("create output directory: %w", err)
	}

	// 1. Try delegating to Archify compiler if available locally
	archifyBin := findArchifyExecutable()
	if archifyBin != "" {
		tempInput := filepath.Join(os.TempDir(), fmt.Sprintf("archify-spec-%d.json", time.Now().UnixNano()))
		if err := os.WriteFile(tempInput, specData, 0644); err == nil {
			defer func() { _ = os.Remove(tempInput) }()

			quality := string(opts.Quality)
			if quality == "" {
				quality = "showcase"
			}

			cmdCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
			defer cancel()

			cmd := exec.CommandContext(cmdCtx, "node", archifyBin, "deliver", string(diagType), tempInput, outClean, "--quality", quality, "--json")
			var stdout, stderr bytes.Buffer
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr

			if err := cmd.Run(); err == nil {
				info, statErr := os.Stat(outClean)
				if statErr == nil && info.Size() > 0 {
					return &RenderResult{
						OutputPath:  outClean,
						DiagramType: string(diagType),
						Title:       spec.Meta.Title,
						ByteCount:   int(info.Size()),
						EngineUsed:  "archify-deliver",
					}, nil
				}
			}
		}
	}

	// 2. Built-in standalone SVG + HTML generator fallback
	htmlContent := generateNativeHtml(spec, diagType)
	if err := os.WriteFile(outClean, []byte(htmlContent), 0644); err != nil {
		return nil, fmt.Errorf("write output HTML %s: %w", outClean, err)
	}

	return &RenderResult{
		OutputPath:  outClean,
		DiagramType: string(diagType),
		Title:       spec.Meta.Title,
		ByteCount:   len(htmlContent),
		EngineUsed:  "cortex-ia-native-svg",
	}, nil
}

func findArchifyExecutable() string {
	candidates := []string{
		"D:\\archify\\archify\\bin\\archify.mjs",
		"D:\\archify\\bin\\archify.mjs",
	}
	home, _ := os.UserHomeDir()
	candidates = append(candidates,
		filepath.Join(home, ".local", "share", "archify", "bin", "archify.mjs"),
		filepath.Join(home, "node_modules", "archify", "bin", "archify.mjs"),
	)
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

func generateNativeHtml(spec DiagramSpecification, diagType DiagramType) string {
	nodes := spec.Components
	if len(nodes) == 0 && len(spec.Steps) > 0 {
		for _, s := range spec.Steps {
			nodes = append(nodes, Component{ID: s.ID, Label: s.Label, Type: s.Type, Sublabel: s.Description})
		}
	}
	if len(nodes) == 0 && len(spec.Participants) > 0 {
		nodes = spec.Participants
	}

	// Layout nodes in a grid
	cols := int(math.Ceil(math.Sqrt(float64(len(nodes)))))
	if cols < 2 {
		cols = 2
	}
	if cols > 5 {
		cols = 5
	}

	boxW := 180
	boxH := 70
	gapX := 80
	gapY := 80
	startX := 60
	startY := 80

	nodeCoords := make(map[string][2]int)
	for i, n := range nodes {
		row := i / cols
		col := i % cols
		x := startX + col*(boxW+gapX)
		y := startY + row*(boxH+gapY)
		nodeCoords[n.ID] = [2]int{x, y}
	}

	rows := (len(nodes) + cols - 1) / cols
	if rows < 1 {
		rows = 1
	}
	svgW := startX*2 + cols*(boxW+gapX) - gapX
	svgH := startY*2 + rows*(boxH+gapY) - gapY
	if svgW < 800 {
		svgW = 800
	}
	if svgH < 500 {
		svgH = 500
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>%s - System Diagram</title>
<style>
:root {
  --bg: #0b0f19;
  --panel: #131b2e;
  --text: #f1f5f9;
  --text-muted: #94a3b8;
  --border: #1e293b;
  --accent: #38bdf8;
  --node-bg: #1e293b;
  --node-border: #334155;
  --edge: #64748b;
  --edge-text: #94a3b8;
}
body {
  margin: 0;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, "Helvetica Neue", sans-serif;
  background: var(--bg);
  color: var(--text);
  overflow-x: auto;
}
.header {
  padding: 1.5rem 2rem;
  background: var(--panel);
  border-bottom: 1px solid var(--border);
  display: flex;
  justify-content: space-between;
  align-items: center;
}
h1 { margin: 0; font-size: 1.4rem; font-weight: 600; display: flex; align-items: center; gap: 0.75rem; }
.badge { background: #0284c7; color: white; padding: 0.2rem 0.6rem; border-radius: 4px; font-size: 0.75rem; font-weight: 600; text-transform: uppercase; }
.theme-btn {
  background: var(--border);
  color: var(--text);
  border: 1px solid var(--node-border);
  padding: 0.4rem 0.8rem;
  border-radius: 6px;
  cursor: pointer;
  font-size: 0.85rem;
}
.theme-btn:hover { background: var(--node-border); }
.canvas-container {
  display: flex;
  justify-content: center;
  align-items: center;
  padding: 2rem;
}
svg {
  background: var(--panel);
  border: 1px solid var(--border);
  border-radius: 8px;
  box-shadow: 0 10px 25px -5px rgba(0,0,0,0.5);
}
.node rect {
  fill: var(--node-bg);
  stroke: var(--node-border);
  stroke-width: 1.5px;
  transition: all 0.2s ease;
  cursor: pointer;
}
.node:hover rect {
  stroke: var(--accent);
  fill: #25334d;
}
.node text.label {
  fill: var(--text);
  font-size: 13px;
  font-weight: 600;
  pointer-events: none;
}
.node text.sublabel {
  fill: var(--text-muted);
  font-size: 11px;
  pointer-events: none;
}
.edge path {
  fill: none;
  stroke: var(--edge);
  stroke-width: 1.5px;
}
.edge text {
  fill: var(--edge-text);
  font-size: 11px;
  text-anchor: middle;
}
</style>
</head>
<body>
<div class="header">
  <div>
    <h1>%s <span class="badge">%s</span></h1>
    <div style="color: var(--text-muted); font-size: 0.85rem; margin-top: 0.25rem;">%s</div>
  </div>
  <button class="theme-btn" onclick="toggleTheme()">Toggle Theme</button>
</div>
<div class="canvas-container">
<svg width="%d" height="%d" viewBox="0 0 %d %d">
<defs>
  <marker id="arrow" viewBox="0 0 10 10" refX="8" refY="5" markerWidth="6" markerHeight="6" orient="auto-start-reverse">
    <path d="M 0 1 L 10 5 L 0 9 z" fill="#64748b" />
  </marker>
</defs>
`, spec.Meta.Title, spec.Meta.Title, diagType, spec.Meta.Subtitle, svgW, svgH, svgW, svgH)

	// Draw Connections
	for _, conn := range spec.Connections {
		fromPt, okFrom := nodeCoords[conn.From]
		toPt, okTo := nodeCoords[conn.To]
		if !okFrom || !okTo {
			continue
		}

		x1 := fromPt[0] + boxW/2
		y1 := fromPt[1] + boxH/2
		x2 := toPt[0] + boxW/2
		y2 := toPt[1] + boxH/2

		// Compute orthogonal path
		midX := (x1 + x2) / 2
		pathD := fmt.Sprintf("M %d %d L %d %d L %d %d", x1, y1, midX, y1, x2, y2)
		if y1 == y2 {
			pathD = fmt.Sprintf("M %d %d L %d %d", x1, y1, x2, y2)
		}

		fmt.Fprintf(&sb, `  <g class="edge">
    <path d="%s" marker-end="url(#arrow)"/>
`, pathD)
		if conn.Label != "" {
			fmt.Fprintf(&sb, `    <text x="%d" y="%d">%s</text>
`, midX, (y1+y2)/2-6, conn.Label)
		}
		sb.WriteString(`  </g>
`)
	}

	// Draw Nodes
	for _, n := range nodes {
		pt := nodeCoords[n.ID]
		x := pt[0]
		y := pt[1]

		subText := n.Type
		if n.Sublabel != "" {
			subText = n.Sublabel
		}

		fmt.Fprintf(&sb, `  <g class="node" id="node-%s">
    <rect x="%d" y="%d" width="%d" height="%d" rx="6" ry="6"/>
    <text class="label" x="%d" y="%d">%s</text>
    <text class="sublabel" x="%d" y="%d">%s</text>
  </g>
`, n.ID, x, y, boxW, boxH, x+12, y+28, n.Label, x+12, y+48, subText)
	}

	sb.WriteString(`</svg>
</div>
<script>
function toggleTheme() {
  const current = document.body.style.getPropertyValue('--bg');
  if (current === '#ffffff') {
    document.body.style.setProperty('--bg', '#0b0f19');
    document.body.style.setProperty('--panel', '#131b2e');
    document.body.style.setProperty('--text', '#f1f5f9');
    document.body.style.setProperty('--text-muted', '#94a3b8');
    document.body.style.setProperty('--border', '#1e293b');
    document.body.style.setProperty('--node-bg', '#1e293b');
    document.body.style.setProperty('--node-border', '#334155');
  } else {
    document.body.style.setProperty('--bg', '#ffffff');
    document.body.style.setProperty('--panel', '#f8fafc');
    document.body.style.setProperty('--text', '#0f172a');
    document.body.style.setProperty('--text-muted', '#64748b');
    document.body.style.setProperty('--border', '#e2e8f0');
    document.body.style.setProperty('--node-bg', '#f1f5f9');
    document.body.style.setProperty('--node-border', '#cbd5e1');
  }
}
</script>
</body>
</html>`)

	return sb.String()
}
