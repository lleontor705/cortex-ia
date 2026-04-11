# Code Review Rules

## General
REJECT if:
- Hardcoded secrets or credentials
- Empty catch blocks (silent error handling)
- Code duplication (violates DRY)
- console.log / print() in production code
- Missing error handling (bare returns)

## Go
REJECT if:
- Exported functions without doc comments
- Ignored errors (bare _ = err in logic paths)
- Naked returns in long functions
- Missing context wrapping in errors

## TypeScript/React
REJECT if:
- var usage (use const/let)
- Missing type annotations on public functions
- Direct DOM manipulation in React components

## Python
REJECT if:
- Missing type hints on public functions
- Bare except: without specific exception
- print() instead of logger in library code

## Response Format
FIRST LINE must be exactly:
STATUS: PASSED
or
STATUS: FAILED

If FAILED, list: file:line - rule violated - issue
