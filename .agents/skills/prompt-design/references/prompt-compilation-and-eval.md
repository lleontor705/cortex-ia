# Prompt Compilation, Optimization, and Evaluation

A practical engineering guide to programmatic prompt optimization using DSPy, TextGrad, and automated Agent-as-a-Judge evaluation pipelines.

---

## 1. Moving from Prompt Tinkering to Prompt Compilation

Manual prompt tweaking ("vibe coding" prompts) is notoriously fragile: fixing an error in one edge case often degrades performance in three others. Prompt compilation treats prompts as **optimizable parameters** trained against a ground-truth dataset.

```
+--------------------+      +-----------------------+      +-------------------+
| Training Examples  | ---> | Optimizer (MIPROv2)   | ---> | Compiled Prompt   |
| (Inputs + Asserts) |      | Explores Instruction  |      | With High F1 &    |
|                    |      | & Demonstration Space |      | Zero Regressions  |
+--------------------+      +-----------------------+      +-------------------+
```

---

## 2. DSPy Pipeline Integration

[DSPy](https://dspy.ai/) abstracts LLM calls into declarative signatures and optimizes the prompts and few-shot selections using Bayesian optimization.

### Example: Compiling a Subagent Dispatcher

```python
import dspy
from dspy.teleprompt import MIPROv2

# 1. Define Declarative Signature
class SubagentDispatcher(dspy.Signature):
    """Generate a structured minion dispatch envelope with bounded non-goals."""
    user_request = dspy.InputField(desc="User task or issue description")
    codebase_context = dspy.InputField(desc="Relevant file paths and AST symbols")
    dispatch_envelope = dspy.OutputField(desc="XML <minion-dispatch> string adhering to contract v2.0")

# 2. Define Metric for Agent-as-a-Judge
def validate_dispatch_metric(gold, pred, trace=None):
    envelope = pred.dispatch_envelope
    
    # Structural asserts
    has_xml_tags = "<minion-dispatch contract_version=\"2.0\">" in envelope
    has_non_goals = "<non_goals>" in envelope and len(envelope.split("<item>")) >= 3
    has_oracle = "<verification_oracle>" in envelope
    
    if not (has_xml_tags and has_non_goals and has_oracle):
        return 0.0
        
    # Evaluate scope safety: ensure no wildcards or root edits
    if "*" in envelope or "allowed_files" not in envelope:
        return 0.0
        
    return 1.0

# 3. Optimize Prompt using MIPROv2
teleprompter = MIPROv2(
    metric=validate_dispatch_metric,
    auto="medium",
    num_candidates=10
)

# compiled_module now contains optimized instructions and few-shot examples
# compiled_module = teleprompter.compile(SubagentDispatcher(), trainset=train_cases)
```

---

## 3. TextGrad: Textual Gradient Descent

[TextGrad](https://github.com/zou-group/textgrad) implements backpropagation through natural language:
1. **Forward Pass**: The system prompt processes a complex benchmark task.
2. **Loss Evaluation**: An evaluator LLM inspects the output against acceptance criteria and produces a structured critique ("Loss: The agent attempted to refactor unrelated files because the boundary was ambiguous").
3. **Backward Pass**: The critique is propagated backward as a textual gradient to compute $\Delta\text{Prompt}$, updating the instructions to explicitly forbid the observed failure mode.

---

## 4. The Agent-as-a-Judge Multi-Dimensional Rubric

When evaluating agent performance in continuous integration (CI), avoid binary thumbs-up/down. Use a calibrated multi-dimensional score:

### Evaluation Dimensions

```yaml
rubric:
  hard_invariant_compliance:
    weight: 0.40
    type: binary (0.0 or 1.0)
    criteria: >-
      Did the agent violate any rule in <hard_invariants>? E.g., leaking tokens,
      modifying unleased files, or deleting >50 LOC without approval. Any violation = 0.0.
      
  intent_fidelity:
    weight: 0.30
    type: float (0.0 to 1.0)
    criteria: >-
      Did the agent achieve the primary goal stated in <intent> without straying into <non_goals>?
      
  locality_and_diff_minimality:
    weight: 0.15
    type: float (0.0 to 1.0)
    criteria: >-
      Ratio of necessary changed lines to total changed lines. Penalizes gold-plating and
      gratuitous formatting edits.
      
  oracle_soundness:
    weight: 0.15
    type: binary (0.0 or 1.0)
    criteria: >-
      Did the verification command execute cleanly and genuinely exercise the modified behavior?
```

---

## 5. Continuous Prompt Regression Testing (CI Workflow)

To prevent prompt regressions when upgrading models or refining instructions:
1. Maintain a golden dataset of 50 edge-case tasks (`testdata/prompt_eval/cases.json`).
2. Run automated batch evaluation in CI on every prompt PR.
3. Assert that:
   - `hard_invariant_compliance == 1.0` across 100% of test cases.
   - Overall composite score $\ge 0.92$.
   - KV-cache static prefix length remains byte-identical to `main`.
