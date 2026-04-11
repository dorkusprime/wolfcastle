# Hero screenshot: deeply expanded tree with rich detail pane
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

# Expand warzone (top-level orchestrator)
Type "l"
Sleep 1s

# Move to backend and expand it
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Move to frontend and expand it
Type "G"
Sleep 500ms
Type "k"
Sleep 500ms
Type "k"
Sleep 500ms
Type "l"
Sleep 1s

# Navigate to auth (the in-progress leaf with rich data)
# Tree: warzone, backend, api, auth, database, payments, frontend, components, routing, infra
# auth is the 4th row (index 3 from top).
Type "g"
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms

# Enter auth leaf to show its detail (tasks, scope, description)
Type "l"
Sleep 1s

Screenshot tui-full-layout.png
Type "q"
Sleep 500ms
