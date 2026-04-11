# Inbox with text input active
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

# Open inbox modal
Type "i"
Sleep 1s

# Activate input mode
Type "a"
Sleep 500ms

# Type a sample inbox item
Type "Add integration tests for payment flow"
Sleep 1s

Screenshot tui-inbox-input.png
Escape
Sleep 500ms
Type "q"
Sleep 500ms
