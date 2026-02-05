# Simtezilo

## Dev environment tips

- Use `gofmt` for general formatting.
- Never use inline error handling.
- Use `make lint` to check for linting issues. Any new changes should pass all linter checks.
- Use testify for test assertions
- Arrange tests in arrange, act, and assert sections
- Name tests using the test[FeatureBeingTested] pattern

## Testing instruction

- Use `make test` to run all tests in the project. All tests must pass for a change to be consodered successful.