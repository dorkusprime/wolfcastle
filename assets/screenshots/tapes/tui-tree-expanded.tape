# Tree pane fully expanded with all orchestrators open, dashboard in detail
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
Sleep 10s
# Sync alt-screen with refresh
Type "R"
Sleep 2s
Type "R"
Sleep 1s

# Expand warzone
Type "l"
Sleep 1s

# Expand backend (first child)
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Navigate past backend's children to frontend and expand it
# Tree: warzone, backend, api, auth, database, frontend, ...
# Go to bottom, up past infra, land on frontend
Type "G"
Sleep 500ms
Type "k"
Sleep 500ms
Type "l"
Sleep 1s

# Return to dashboard view and keep focus on tree
Type "d"
Sleep 1s

Screenshot tui-tree-expanded.png
Type "q"
Sleep 500ms
