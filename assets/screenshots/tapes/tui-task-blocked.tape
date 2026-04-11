# Blocked task with failure count and block reason
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

# Expand warzone
Type "l"
Sleep 1s

# Move to backend (first child) and expand
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Move down past api, auth, database to payments (blocked)
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms

# Enter payments leaf to see tasks
Type "l"
Sleep 1s

# Select the first task (blocked Stripe integration)
Type "l"
Sleep 1s

Screenshot tui-task-blocked.png
Type "q"
Sleep 500ms
