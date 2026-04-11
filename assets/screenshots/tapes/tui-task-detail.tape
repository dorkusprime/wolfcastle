# Task detail: navigate into a leaf, expand tasks, enter a task
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

# Move to backend and expand
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Navigate to auth (skip api)
Type "g"
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms

# Expand auth leaf to show task rows
Type "l"
Sleep 1s

# Move down to first task "Add OAuth2 PKCE flow"
Type "j"
Sleep 500ms

# Enter task detail
Type "l"
Sleep 1s

Screenshot tui-task-detail.png
Type "q"
Sleep 500ms
