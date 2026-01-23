# Design Decisions

## 1. Intermediate Representation (OPF)

**Decision**: We use an `opf.OEBBook` struct as an intermediate representation between parsing and writing.

**Context**: different output formats (MOBI, EPUB) share many commonalities, specifically the need for metadata, a manifest of resources, and a linear reading order.

**Benefits**:

* Decouples input from output. Adding a new input format (e.g., Markdown) would only require mapping it to `OEBBook`.
* Simplifies writers. The MOBI and EPUB writers don't need to know anything about FB2 specifics.

## 2. Standard Library `html/template` vs Text Construction

**Decision**: We largely construct HTML via string builders or specialized transformers rather than complex `html/template` execution for the main body.

**Context**: FB2 is structure-heavy. Efficiently transforming thousands of paragraphs requires performance.

**Benefits**:

* Performance.
* Fine-grained control over the output HTML, which is critical for MOBI compatibility (older Kindles have very limited HTML support).

## 3. `slog` for Logging

**Decision**: Use Go's standard `log/slog` (introduced in Go 1.21).

**Context**: We need structured logging for debugging, but want to avoid heavy external dependencies like `zap` or `logrus` for a simple CLI tool.

**Benefits**:

* Standard library (no extra deps).
* Structured output support (good for parsing logs if needed).
* Level support (Debug/Info/Error).

## 4. Single Binary CLI

**Decision**: The tool is distributed as a single static binary.

**Context**: Users simply want to convert files. They shouldn't need Python runtimes, external DLLs, or complex installation steps.

**Benefits**:

* Ease of distribution and use.
