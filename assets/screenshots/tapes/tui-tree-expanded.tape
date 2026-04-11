# Tree pane with multi-level expanded nodes, mix of glyphs
Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set FontFamily "Bitstream Vera Sans Mono"
Set Padding 20
Set Theme "Dracula"
Set WindowBar Colorful
Set TypingSpeed 0

Type "wolfcastle"
Enter
Sleep 8s
Down
Up
Sleep 1s

# Expand warzone orchestrator
Type "l"
Sleep 1s

# Move down to backend and expand it
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Move down to frontend and expand it
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

Screenshot tui-tree-expanded.png
Type "q"
Sleep 500ms
