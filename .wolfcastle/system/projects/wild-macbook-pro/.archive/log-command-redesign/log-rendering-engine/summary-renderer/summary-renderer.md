# Summary Renderer

The default output mode for wolfcastle log. In replay mode, prints one line per completed stage showing stage type, node address, and duration with success/failure glyphs. In follow mode, prints a start line (▶) when a stage begins and a completion line (✓ or ✗) with duration when it finishes. Also renders audit report paths indented below audit completion lines. Consumes Records from the foundation reader and writes formatted text to an io.Writer.
