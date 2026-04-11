# Blocked task with failure count and block reason
Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set FontFamily "JetBrains Mono"
Set Padding 20
Set Theme "Dracula"
Set WindowBar Colorful
Set TypingSpeed 0

Type "wolfcastle"
Enter
Sleep 8s

# Expand warzone
Type "l"
Sleep 1s

# Move down past backend to frontend
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Expand frontend
Type "l"
Sleep 1s

# Move down past components to routing (blocked)
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Enter routing leaf to see tasks
Type "l"
Sleep 1s

# Select the first task (the blocked one, not audit)
Type "l"
Sleep 1s

Screenshot tui-task-blocked.png
Type "q"
Sleep 500ms
