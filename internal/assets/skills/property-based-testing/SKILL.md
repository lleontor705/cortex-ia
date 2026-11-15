---
name: property-based-testing
description: Generate invariant-driven generative test suites and fuzz vectors using Hypothesis, fast-check, or language equivalents. Use for critical parsers, serializers, and algorithms where example tests miss edge cases.
license: MIT
metadata:
  author: OpenCode Engine
  version: "1.0.0"
---

# Property-based testing

Use this skill for parsers, serializers, auth decoders, data transforms, and boundary-heavy algorithms, where a few hand-picked examples miss edge cases. Keep example tests for the documented cases; add properties for the invariants.

## Common properties

- **Roundtrip**: `decode(encode(x)) == x`.
- **Idempotence**: `f(f(x)) == f(x)`.
- **Oracle**: an optimised `f` matches a simple reference `f` on every input.
- **Invariant**: sorting keeps the length and produces ordered output.
- **No crash**: arbitrary UTF-8, empty input, NUL bytes, and boundary values return an error or a result, never an unhandled panic or exception.

## Examples

TypeScript with `fast-check`:

```typescript
import fc from 'fast-check';

test('JSON roundtrip preserves records', () => {
  fc.assert(
    fc.property(fc.record({ id: fc.uuid(), val: fc.string() }), (data) => {
      expect(JSON.parse(JSON.stringify(data))).toEqual(data);
    })
  );
});
```

Python with Hypothesis:

```python
from hypothesis import given, strategies as st

@given(st.lists(st.integers()))
def test_sort_preserves_length_and_order(xs):
    out = custom_sort(xs)
    assert len(out) == len(xs)
    assert all(out[i] <= out[i + 1] for i in range(len(out) - 1))
```

## Practice

- Write at least one property for each parser, serializer, or cryptographic/data transform in the change.
- Keep the sample count moderate (about 50 to 100 runs) during iteration so the test stays fast. Raise it in CI if needed.
- When a property fails, minimise the input the framework reports and add it as a fixed example test.
