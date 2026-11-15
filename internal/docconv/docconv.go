package docconv

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Format represents an identified document format.
type Format string

const (
	FormatDocx  Format = "docx"
	FormatDoc   Format = "doc"
	FormatPptx  Format = "pptx"
	FormatPpt   Format = "ppt"
	FormatXlsx  Format = "xlsx"
	FormatXls   Format = "xls"
	FormatPdf   Format = "pdf"
	FormatOdt   Format = "odt"
	FormatOds   Format = "ods"
	FormatOdp   Format = "odp"
	FormatRtf   Format = "rtf"
	FormatEpub  Format = "epub"
	FormatCsv   Format = "csv"
	FormatTsv   Format = "tsv"
	FormatText  Format = "text"
	FormatMd    Format = "markdown"
	FormatOther Format = "unknown"
)

// ConvertOptions controls document conversion.
type ConvertOptions struct {
	BeforeWrite func(string) error // Controller-owned authorization, rechecked after conversion.
	FilePath    string
	OutputPath  string
	Format      Format
	MaxLines    int
	OCRMode     string // "reject" or "hosted"
	APIKey      string
	Timeout     time.Duration
}

// ConvertResult contains the converted Markdown and extraction metrics.
type ConvertResult struct {
	FilePath   string `json:"file_path"`
	Format     string `json:"format"`
	Markdown   string `json:"markdown"`
	ByteCount  int    `json:"byte_count"`
	LineCount  int    `json:"line_count"`
	Truncated  bool   `json:"truncated"`
	OutputPath string `json:"output_path,omitempty"`
	EngineUsed string `json:"engine_used"`
}

// InspectResult contains metadata and structure summary without loading full content.
type InspectResult struct {
	FilePath       string   `json:"file_path"`
	Format         string   `json:"format"`
	SizeBytes      int64    `json:"size_bytes"`
	EstimatedUnits int      `json:"estimated_units,omitempty"`
	UnitType       string   `json:"unit_type,omitempty"`
	Headings       []string `json:"headings,omitempty"`
	Summary        string   `json:"summary"`
}

// DetectFormat detects the document format using magic bytes first, falling back to extension.
func DetectFormat(filePath string, data []byte) Format {
	if len(data) >= 5 && bytes.HasPrefix(data, []byte("%PDF-")) {
		return FormatPdf
	}
	if len(data) >= 5 && bytes.HasPrefix(data, []byte("{\\rtf")) {
		return FormatRtf
	}
	// OLE Compound Document (legacy .doc, .xls, .ppt)
	if len(data) >= 8 && bytes.Equal(data[:8], []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}) {
		ext := strings.ToLower(filepath.Ext(filePath))
		switch ext {
		case ".xls":
			return FormatXls
		case ".ppt", ".pps", ".pot":
			return FormatPpt
		default:
			return FormatDoc
		}
	}
	// ZIP-based packages (.docx, .pptx, .xlsx, .odt, .odp, .ods, .epub)
	if len(data) >= 4 && bytes.Equal(data[:4], []byte{0x50, 0x4B, 0x03, 0x04}) {
		if fmtFromZip := detectZipFormat(data); fmtFromZip != FormatOther {
			return fmtFromZip
		}
	}

	// Fallback to extension
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filePath), "."))
	switch ext {
	case "docx", "docm":
		return FormatDocx
	case "doc":
		return FormatDoc
	case "pptx", "pptm", "ppsx", "ppsm":
		return FormatPptx
	case "ppt", "pps", "pot":
		return FormatPpt
	case "xlsx", "xlsm", "xlsb":
		return FormatXlsx
	case "xls":
		return FormatXls
	case "pdf":
		return FormatPdf
	case "odt":
		return FormatOdt
	case "ods":
		return FormatOds
	case "odp":
		return FormatOdp
	case "rtf":
		return FormatRtf
	case "epub":
		return FormatEpub
	case "csv":
		return FormatCsv
	case "tsv":
		return FormatTsv
	case "txt":
		return FormatText
	case "md", "markdown":
		return FormatMd
	default:
		return FormatOther
	}
}

func detectZipFormat(data []byte) Format {
	r, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return FormatOther
	}
	for _, f := range r.File {
		name := strings.ToLower(f.Name)
		if strings.HasPrefix(name, "word/") {
			return FormatDocx
		}
		if strings.HasPrefix(name, "xl/") {
			return FormatXlsx
		}
		if strings.HasPrefix(name, "ppt/") {
			return FormatPptx
		}
		if name == "mimetype" {
			rc, err := f.Open()
			if err == nil {
				mimeBytes, _ := io.ReadAll(io.LimitReader(rc, 256))
				_ = rc.Close()
				mime := string(mimeBytes)
				if strings.Contains(mime, "opendocument.text") {
					return FormatOdt
				}
				if strings.Contains(mime, "opendocument.spreadsheet") {
					return FormatOds
				}
				if strings.Contains(mime, "opendocument.presentation") {
					return FormatOdp
				}
				if strings.Contains(mime, "epub+zip") {
					return FormatEpub
				}
			}
		}
	}
	return FormatOther
}

// Convert processes the specified document and produces clean GitHub-Flavored Markdown.
func Convert(ctx context.Context, opts ConvertOptions) (*ConvertResult, error) {
	if opts.FilePath == "" {
		return nil, errors.New("file_path is required")
	}

	cleanPath := filepath.Clean(opts.FilePath)
	fileInfo, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("stat file %s: %w", cleanPath, err)
	}
	if fileInfo.IsDir() {
		return nil, fmt.Errorf("path %s is a directory, not a document file", cleanPath)
	}

	headerData := make([]byte, 4096)
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", cleanPath, err)
	}
	n, _ := f.Read(headerData)
	_ = f.Close()

	format := opts.Format
	if format == "" || format == FormatOther {
		format = DetectFormat(cleanPath, headerData[:n])
	}

	var mdContent string
	var engineUsed string

	switch format {
	case FormatCsv:
		mdContent, err = convertCsv(cleanPath, ',')
		engineUsed = "native-csv"
	case FormatTsv:
		mdContent, err = convertCsv(cleanPath, '\t')
		engineUsed = "native-tsv"
	case FormatText, FormatMd:
		rawBytes, readErr := os.ReadFile(cleanPath)
		if readErr != nil {
			return nil, fmt.Errorf("read text file: %w", readErr)
		}
		mdContent = string(rawBytes)
		engineUsed = "native-text"
	default:
		// Attempt conversion via anydoc / npx @firecrawl/anydoc
		mdContent, engineUsed, err = convertViaExternalOrFallback(ctx, cleanPath, format, opts)
	}

	if err != nil {
		return nil, err
	}

	// Apply line limit if specified
	lines := strings.Split(mdContent, "\n")
	lineCount := len(lines)
	truncated := false

	if opts.MaxLines > 0 && lineCount > opts.MaxLines {
		mdContent = strings.Join(lines[:opts.MaxLines], "\n") + "\n\n... [Content truncated: reached max_lines limit] ..."
		truncated = true
	}

	// Write to output file if requested
	if opts.OutputPath != "" {
		outClean := filepath.Clean(opts.OutputPath)
		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(outClean); err != nil {
				return nil, err
			}
		}
		if err := os.MkdirAll(filepath.Dir(outClean), 0755); err != nil {
			return nil, fmt.Errorf("create output directory: %w", err)
		}
		if opts.BeforeWrite != nil {
			if err := opts.BeforeWrite(outClean); err != nil {
				return nil, err
			}
		}
		if err := os.WriteFile(outClean, []byte(mdContent), 0644); err != nil {
			return nil, fmt.Errorf("write output markdown %s: %w", outClean, err)
		}
	}

	return &ConvertResult{
		FilePath:   cleanPath,
		Format:     string(format),
		Markdown:   mdContent,
		ByteCount:  len(mdContent),
		LineCount:  lineCount,
		Truncated:  truncated,
		OutputPath: opts.OutputPath,
		EngineUsed: engineUsed,
	}, nil
}

// Inspect returns structural metadata for the file without expanding complete text.
func Inspect(filePath string) (*InspectResult, error) {
	cleanPath := filepath.Clean(filePath)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("stat file %s: %w", cleanPath, err)
	}

	headerData := make([]byte, 4096)
	f, err := os.Open(cleanPath)
	if err != nil {
		return nil, fmt.Errorf("open file %s: %w", cleanPath, err)
	}
	n, _ := f.Read(headerData)
	_ = f.Close()

	format := DetectFormat(cleanPath, headerData[:n])
	res := &InspectResult{
		FilePath:  cleanPath,
		Format:    string(format),
		SizeBytes: info.Size(),
	}

	switch format {
	case FormatCsv, FormatTsv:
		res.UnitType = "rows"
		res.EstimatedUnits = countFileLines(cleanPath)
		res.Summary = fmt.Sprintf("Delimited tabular file (%s) with approximately %d rows", format, res.EstimatedUnits)
	case FormatDocx:
		res.UnitType = "sections"
		res.Headings, res.EstimatedUnits = inspectDocxHeadings(cleanPath)
		res.Summary = fmt.Sprintf("WordprocessingML Document (.docx), size %d bytes, contains %d headings/sections", info.Size(), len(res.Headings))
	case FormatXlsx:
		sheets := inspectXlsxSheets(cleanPath)
		res.UnitType = "sheets"
		res.EstimatedUnits = len(sheets)
		res.Headings = sheets
		res.Summary = fmt.Sprintf("Spreadsheet Workbook (.xlsx), %d sheets: %s", len(sheets), strings.Join(sheets, ", "))
	case FormatPptx:
		slides := countPptxSlides(cleanPath)
		res.UnitType = "slides"
		res.EstimatedUnits = slides
		res.Summary = fmt.Sprintf("Presentation (.pptx) with %d slides", slides)
	case FormatPdf:
		pages := estimatePdfPages(cleanPath)
		res.UnitType = "pages"
		res.EstimatedUnits = pages
		res.Summary = fmt.Sprintf("PDF Document, size %d bytes, approximately %d pages", info.Size(), pages)
	default:
		res.UnitType = "lines"
		res.EstimatedUnits = countFileLines(cleanPath)
		res.Summary = fmt.Sprintf("Document format %s, size %d bytes", format, info.Size())
	}

	return res, nil
}

func convertCsv(filePath string, delim rune) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delim
	r.LazyQuotes = true
	r.FieldsPerRecord = -1

	var sb strings.Builder
	rowIdx := 0

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			sb.WriteString(strings.Join(record, " | "))
			sb.WriteString("\n")
			continue
		}

		if rowIdx == 0 {
			sb.WriteString("| ")
			sb.WriteString(strings.Join(escapePipes(record), " | "))
			sb.WriteString(" |\n|")
			for range record {
				sb.WriteString(" --- |")
			}
			sb.WriteString("\n")
		} else {
			sb.WriteString("| ")
			sb.WriteString(strings.Join(escapePipes(record), " | "))
			sb.WriteString(" |\n")
		}
		rowIdx++
	}

	return sb.String(), nil
}

func escapePipes(row []string) []string {
	out := make([]string, len(row))
	for i, s := range row {
		clean := strings.ReplaceAll(s, "|", "\\|")
		clean = strings.ReplaceAll(clean, "\n", " ")
		out[i] = strings.TrimSpace(clean)
	}
	return out
}

func convertViaExternalOrFallback(ctx context.Context, filePath string, format Format, opts ConvertOptions) (string, string, error) {
	// 1. Try anydoc executable if found in PATH or configured paths
	anydocBin := findExecutable("anydoc")
	if anydocBin != "" {
		args := []string{filePath}
		if opts.Format != "" {
			args = append(args, "--format", string(opts.Format))
		}
		if opts.OCRMode != "" {
			args = append(args, "--ocr", opts.OCRMode)
		}
		if opts.APIKey != "" {
			args = append(args, "--api-key", opts.APIKey)
		}
		cmd := exec.CommandContext(ctx, anydocBin, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil && stdout.Len() > 0 {
			return stdout.String(), "anydoc-cli", nil
		}
	}

	// 2. Try npx -y @firecrawl/anydoc
	npxBin := findExecutable("npx")
	if npxBin == "" {
		npxBin = findExecutable("npx.cmd")
	}
	if npxBin != "" {
		args := []string{"-y", "@firecrawl/anydoc", filePath}
		if opts.Format != "" {
			args = append(args, "--format", string(opts.Format))
		}
		if opts.OCRMode != "" {
			args = append(args, "--ocr", opts.OCRMode)
		}
		if opts.APIKey != "" {
			args = append(args, "--api-key", opts.APIKey)
		}
		timeout := opts.Timeout
		if timeout == 0 {
			timeout = 60 * time.Second
		}
		cmdCtx, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()

		cmd := exec.CommandContext(cmdCtx, npxBin, args...)
		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		if err := cmd.Run(); err == nil && stdout.Len() > 0 {
			return stdout.String(), "npx-anydoc", nil
		}
	}

	// 3. Native Go fallback for XML-based office formats
	switch format {
	case FormatDocx:
		content, err := extractDocxText(filePath)
		if err == nil && content != "" {
			return content, "native-docx-fallback", nil
		}
	case FormatXlsx:
		content, err := extractXlsxText(filePath)
		if err == nil && content != "" {
			return content, "native-xlsx-fallback", nil
		}
	}

	return "", "", fmt.Errorf("unable to convert document format %s: anydoc / npx tool unavailable or returned empty output", format)
}

func findExecutable(name string) string {
	if p, err := exec.LookPath(name); err == nil {
		return p
	}
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, ".cargo", "bin", name),
		filepath.Join(home, ".cargo", "bin", name+".exe"),
		filepath.Join("C:\\Program Files\\Volta", name),
		filepath.Join("C:\\Program Files\\Volta", name+".cmd"),
		filepath.Join("C:\\Program Files\\nodejs", name),
		filepath.Join("C:\\Program Files\\nodejs", name+".cmd"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c
		}
	}
	return ""
}

var xmlTagRegex = regexp.MustCompile(`<[^>]+>`)

func extractDocxText(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			data, err := io.ReadAll(rc)
			_ = rc.Close()
			if err != nil {
				return "", err
			}
			return parseDocxXml(data), nil
		}
	}
	return "", errors.New("document.xml not found in docx")
}

func parseDocxXml(data []byte) string {
	xmlStr := string(data)
	xmlStr = strings.ReplaceAll(xmlStr, "</w:p>", "\n\n")
	xmlStr = strings.ReplaceAll(xmlStr, "<w:br/>", "\n")
	xmlStr = strings.ReplaceAll(xmlStr, "</w:tr>", "\n")
	xmlStr = strings.ReplaceAll(xmlStr, "</w:tc>", " | ")

	clean := xmlTagRegex.ReplaceAllString(xmlStr, "")
	clean = strings.ReplaceAll(clean, "&amp;", "&")
	clean = strings.ReplaceAll(clean, "&lt;", "<")
	clean = strings.ReplaceAll(clean, "&gt;", ">")
	clean = strings.ReplaceAll(clean, "&quot;", "\"")
	clean = strings.ReplaceAll(clean, "&apos;", "'")

	lines := strings.Split(clean, "\n")
	var result []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}
	return strings.Join(result, "\n\n")
}

func extractXlsxText(filePath string) (string, error) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return "", err
	}
	defer func() { _ = r.Close() }()

	var sharedStrings []string
	for _, f := range r.File {
		if f.Name == "xl/sharedStrings.xml" {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				_ = rc.Close()
				matches := regexp.MustCompile(`<t[^>]*>([^<]*)</t>`).FindAllStringSubmatch(string(data), -1)
				for _, m := range matches {
					if len(m) > 1 {
						sharedStrings = append(sharedStrings, m[1])
					}
				}
			}
			break
		}
	}

	var sb strings.Builder
	sb.WriteString("# Spreadsheet Content\n\n")
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "xl/worksheets/sheet") && strings.HasSuffix(f.Name, ".xml") {
			fmt.Fprintf(&sb, "## %s\n\n", filepath.Base(f.Name))
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				_ = rc.Close()
				rows := regexp.MustCompile(`<row[^>]*>(.*?)</row>`).FindAllStringSubmatch(string(data), -1)
				for rIdx, row := range rows {
					if len(row) > 1 {
						cells := regexp.MustCompile(`<v>([^<]*)</v>`).FindAllStringSubmatch(row[1], -1)
						var rowVals []string
						for _, c := range cells {
							if len(c) > 1 {
								val := c[1]
								var idx int
								if _, err := fmt.Sscanf(val, "%d", &idx); err == nil && idx >= 0 && idx < len(sharedStrings) {
									val = sharedStrings[idx]
								}
								rowVals = append(rowVals, val)
							}
						}
						if len(rowVals) > 0 {
							sb.WriteString("| " + strings.Join(escapePipes(rowVals), " | ") + " |\n")
							if rIdx == 0 {
								sb.WriteString("|" + strings.Repeat(" --- |", len(rowVals)) + "\n")
							}
						}
					}
				}
				sb.WriteString("\n")
			}
		}
	}
	return sb.String(), nil
}

func countFileLines(filePath string) int {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}
	return bytes.Count(data, []byte("\n")) + 1
}

func inspectDocxHeadings(filePath string) ([]string, int) {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil, 0
	}
	defer func() { _ = r.Close() }()

	var headings []string
	sections := 1
	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(rc)
			_ = rc.Close()

			matches := regexp.MustCompile(`<w:pStyle\s+w:val="Heading(\d+)"[^>]*>.*?<w:t[^>]*>([^<]+)</w:t>`).FindAllStringSubmatch(string(data), -1)
			for _, m := range matches {
				if len(m) > 2 {
					headings = append(headings, fmt.Sprintf("H%s: %s", m[1], m[2]))
				}
			}
			sections = len(headings)
			if sections == 0 {
				sections = 1
			}
			break
		}
	}
	return headings, sections
}

func inspectXlsxSheets(filePath string) []string {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return nil
	}
	defer func() { _ = r.Close() }()

	var sheets []string
	for _, f := range r.File {
		if f.Name == "xl/workbook.xml" {
			rc, err := f.Open()
			if err == nil {
				data, _ := io.ReadAll(rc)
				_ = rc.Close()
				matches := regexp.MustCompile(`<sheet\s+name="([^"]+)"`).FindAllStringSubmatch(string(data), -1)
				for _, m := range matches {
					if len(m) > 1 {
						sheets = append(sheets, m[1])
					}
				}
			}
			break
		}
	}
	if len(sheets) == 0 {
		sheets = []string{"Sheet1"}
	}
	return sheets
}

func countPptxSlides(filePath string) int {
	r, err := zip.OpenReader(filePath)
	if err != nil {
		return 0
	}
	defer func() { _ = r.Close() }()

	slides := 0
	for _, f := range r.File {
		if strings.HasPrefix(f.Name, "ppt/slides/slide") && strings.HasSuffix(f.Name, ".xml") {
			slides++
		}
	}
	return slides
}

func estimatePdfPages(filePath string) int {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0
	}
	matches := regexp.MustCompile(`/Type\s*/Page\b`).FindAllIndex(data, -1)
	return len(matches)
}
