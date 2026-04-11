# MANUAL: Requires a running daemon with parallel mode. Capture manually.
# Dashboard showing parallel workers.
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
Sleep 10s

# Nudge cursor to sync alt-screen buffer
Down
Sleep 1s
Up
Sleep 1s

Screenshot tui-parallel-dashboard.png
Type "q"
Sleep 500ms
