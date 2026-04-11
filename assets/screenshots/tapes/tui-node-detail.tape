# Node detail view: press Enter on a tree node
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

# Expand warzone, then select backend orchestrator to show node detail
Type "l"
Sleep 1s
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

Screenshot tui-node-detail.png
Type "q"
Sleep 500ms
