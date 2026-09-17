# tfo-1.2 standalone CLI fixture alignment

This additive amendment applies only to tfo-1.2 and REQ-TOOLS-002. Preserve the four original shared pins and every other task binding.

Add `internal/app/doc_diagram_test.go` to the ten existing allowed paths. Modify only the existing output-producing calls in TestCLIDoc and TestCLIDiagram: pass `--standalone` explicitly for document conversion to out.md and diagram rendering to arch.html. These isolated temporary-directory fixtures intentionally exercise standalone utilities rather than an agent controller. Do not add a test suite, weaken output assertions, or enable standalone mode in host-controlled tool dispatch.

The original acceptance remains unchanged: controller writes require trusted context, live lease and prewrite revalidation; standalone CLI intent is explicit. This amendment supersedes only the ten-path scope enumeration by adding one fixture file. Run the existing task checks and actual isolated authority harness. Real work authority continues through the installed absolute schema-13 CLI until final rollout.
