# MANUAL: Requires a running daemon. Capture manually.
# Toast notification in upper-right triggered by state changes.
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

# Nudge cursor to sync alt-screen buffer
Down
Sleep 500ms
Up
Sleep 1s

Screenshot tui-toast.png
Type "q"
Sleep 500ms
