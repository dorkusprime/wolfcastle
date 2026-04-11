# Log modal with level filter cycled to WARN+
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

# Open log modal
Type "L"
Sleep 1s

# Cycle level filter: all -> debug -> info -> warn
Type "L"
Sleep 500ms
Type "L"
Sleep 500ms
Type "L"
Sleep 1s

Screenshot tui-log-filtered.png
Type "q"
Sleep 500ms
