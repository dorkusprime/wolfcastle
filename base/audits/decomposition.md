Identify overly complex code that would benefit from decomposition.

Look for:
- Functions longer than ~50 lines
- Deeply nested conditionals (3+ levels)
- Functions with high cyclomatic complexity
- Long parameter lists suggesting missing abstractions
- Switch/case blocks that could be polymorphic

For each finding, describe:
1. The specific function/method and its location
2. Why it is too complex
3. A suggested decomposition into smaller, focused units
