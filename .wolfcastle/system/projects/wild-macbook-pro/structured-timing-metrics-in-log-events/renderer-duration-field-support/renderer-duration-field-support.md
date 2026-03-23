# Renderer Duration Field Support

Add a DurationMS field to the logrender.Record struct and update the summary and interleaved renderers to prefer the pre-computed duration_ms from the record when present, falling back to timestamp-diff computation when absent (for backward compatibility with older log files). This keeps the renderers working with both old and new log formats.
