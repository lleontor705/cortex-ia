---
name: document-reader
description: "Inspect and convert office documents (.docx, .xlsx, .pptx, .pdf, .odt, .rtf, .epub, .csv) into clean GitHub-Flavored Markdown for LLMs and agents. Use when a task names an office, PDF, or ebook file that is not plain text."
license: MIT
metadata:
  author: cortex-ia
  version: "1.0.0"
---

# Document Reader

Use `cortex_ia_doc_convert` to turn a document that is not plain text into GitHub-flavored Markdown, then read the Markdown like any other file.

| Category | Extensions |
|---|---|
| Word processing | `.docx`, `.doc`, `.docm`, `.odt`, `.rtf` |
| Spreadsheets | `.xlsx`, `.xls`, `.xlsm`, `.xlsb`, `.ods`, `.csv`, `.tsv` |
| Presentations | `.pptx`, `.ppt`, `.ppsx`, `.odp` |
| Documents and ebooks | `.pdf`, `.epub` |

## Keeping context small

- For a document longer than about 10 pages, convert to a file with `output_path` (for example `docs/spec.md`), then search it with `grep` or read the sections you need.
- To skim an excerpt, pass `max_lines: 300`.
- For an image-only scanned PDF, pass `ocr: "hosted"`. That sends the file to the hosted OCR service, so confirm the document may leave the machine first.
