# Hero screenshot: header + tree + dashboard + footer
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

# Expand the warzone orchestrator to show children in tree
Type "l"
Sleep 2s

Screenshot tui-full-layout.png
Type "q"
Sleep 500ms
