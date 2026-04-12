# "Blocked on all fronts" state with all nodes blocked
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

# Sync alt-screen: open and close help overlay to force full redraws
Type "R"
Sleep 2s
Type "R"
Sleep 2s

Screenshot tui-all-blocked.png
Type "q"
Sleep 500ms
