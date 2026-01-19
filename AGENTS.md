# Agent Rules

1. **Never enable compression**: The compression implementation in `mobi` package is broken and produces garbage output. Always use `NoCompression` (default) for MOBI generation. Do not attempt to fix or enable `PalmDOCCompression` unless explicitly instructed to debug it *and* verification proves it works.
