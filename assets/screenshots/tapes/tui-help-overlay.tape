# Help overlay showing all keybinding sections
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

# Open help overlay
Type "?"
Sleep 1s

Screenshot tui-help-overlay.png
Type "?"
Sleep 500ms
Type "q"
Sleep 500ms
