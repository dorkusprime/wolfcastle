# Hero screenshot: expanded tree with task detail showing rich data
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

# Expand warzone
Type "l"
Sleep 1s

# Move to backend and expand it
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Navigate to auth (row 3 from top: warzone, backend, api, auth)
Type "g"
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms

# Expand auth leaf to show its task rows in the tree
Type "l"
Sleep 1s

# Move down to the first task row "Add OAuth2 PKCE flow"
Type "j"
Sleep 500ms

# Enter the task to show task detail (description, deliverables, scope)
Type "l"
Sleep 1s

Screenshot tui-full-layout.png
Type "q"
Sleep 500ms
