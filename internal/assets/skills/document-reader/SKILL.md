---
name: document-reader
description: "Inspect and convert office documents (.docx, .xlsx, .pptx, .pdf, .odt, .rtf, .epub, .csv) into clean GitHub-Flavored Markdown for LLMs and agents."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---

# Document Reader & Converter (Inspired by Anydoc)

Use this skill when dealing with office documents, spreadsheets, presentations, ebooks, or PDF specifications that cannot be read as plain text source code.

## 1. Core Tools

- `cortex_ia_doc_convert`: Convert document to clean GitHub-Flavored Markdown.

## 2. Supported Formats

| Category | Extensions |
|---|---|
| **Word Processing** | `.docx`, `.doc`, `.docm`, `.odt`, `.rtf` |
| **Spreadsheets** | `.xlsx`, `.xls`, `.xlsm`, `.xlsb`, `.ods`, `.csv`, `.tsv` |
| **Presentations** | `.pptx`, `.ppt`, `.ppsx`, `.odp` |
| **Documents & Ebooks** | `.pdf`, `.epub` |

## 3. Best Practices & Token Preservation

1. **Use `output_path` for Multi-Page Manuals**:
   If a PDF or document is over 10 pages, convert to a file (`output_path: "docs/spec.md"`) and use standard reading tools (`read`, `grep`) on the generated Markdown.
3. **Limit Lines**:
   Set `max_lines: 300` when quickly scanning an excerpt in context.
4. **Scanned PDFs**:
   For image-only scanned PDFs, pass `ocr: "hosted"` to process via OCR parse service.
