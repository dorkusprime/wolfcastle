# Node detail view: backend orchestrator showing children and states
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

# Move to backend orchestrator
Type "j"
Sleep 500ms

# Enter backend to show its node detail (children list with states)
Type "l"
Sleep 1s

Screenshot tui-node-detail.png
Type "q"
Sleep 500ms
