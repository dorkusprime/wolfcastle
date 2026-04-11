# Blocked task with failure count and block reason (uses task-blocked stage)
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

# Move to payments (only child) and enter it
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Select the first task (blocked Stripe integration)
Type "l"
Sleep 1s

Screenshot tui-task-blocked.png
Type "q"
Sleep 500ms
