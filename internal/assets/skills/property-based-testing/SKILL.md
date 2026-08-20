---
name: property-based-testing
description: Generate invariant-driven generative test suites and fuzz vectors using Hypothesis, fast-check, or language equivalents.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# Property-Based Testing & Generative Invariant Verification

Use this skill when implementing critical algorithms, parsers, serializers, auth decoders, data transforms, and boundary conditions where fixed few-example unit tests fail to expose subtle edge cases.

## 1. Core Property Paradigms

Instead of `assert f("sample") == "expected"`, test mathematical invariants across hundreds of pseudo-random inputs:

1. **Roundtrip / Invertibility:**
   $$\forall x : \text{deserialize}(\text{serialize}(x)) == x$$
2. **Idempotence:**
   $$\forall x : f(f(x)) == f(x)$$
3. **Equivalence / Oracle:**
   $$\forall x : f_{\text{optimized}}(x) == f_{\text{reference}}(x)$$
4. **Invariant Preservation:**
   $$\forall x : \text{len}(\text{sort}(x)) == \text{len}(x) \land \text{is\_sorted}(\text{sort}(x))$$
5. **No-Crash / Fuzz Invariant:**
   $$\forall x : f(x) \text{ handles arbitrary UTF-8, NULL, and boundary values without unhandled exceptions}$$

## 2. Tooling Reference

### TypeScript / JavaScript: `fast-check`
```typescript
import fc from 'fast-check';

test('roundtrip JSON encode-decode invariant', () => {
  fc.assert(
    fc.property(fc.record({ id: fc.uuid(), val: fc.string() }), (data) => {
      expect(JSON.parse(JSON.stringify(data))).toEqual(data);
    })
  );
});
```

### Python: `Hypothesis`
```python
from hypothesis import given, strategies as st

@given(st.lists(st.integers()))
def test_sort_preserves_length_and_order(xs):
    sorted_xs = custom_sort(xs)
    assert len(sorted_xs) == len(xs)
    assert all(sorted_xs[i] <= sorted_xs[i + 1] for i in range(len(sorted_xs) - 1))
```

## 3. Minion Execution Strategy
- For cryptographic, parsing, or data-transformation tasks, write at least 1 property-based invariant test.
- Check execution time: Keep generated sample count moderate (e.g. 50-100 runs) during iterative loops to maintain sub-second turnaround.
