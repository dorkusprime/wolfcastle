# Dashboard with node counts and progress info
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

# Switch to dashboard view
Type "d"
Sleep 1s

Screenshot tui-dashboard-active.png
Type "q"
Sleep 500ms
