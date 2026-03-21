# Interleaved Renderer

The --interleaved output mode and the renderer shared with non-daemon stdout streaming. Prints stage headers with wall-clock timestamps and glyphs, interspersed with indented agent output, all in chronological order. This is the most complex renderer because it interleaves two record streams (stage events and assistant text) and formats timestamps. Must work in both replay and follow modes.
