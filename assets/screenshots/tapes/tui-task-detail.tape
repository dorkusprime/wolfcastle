# Task detail: navigate into a leaf, then into a task
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

# Expand warzone > backend > auth (in_progress leaf with tasks)
Type "l"
Sleep 1s
Type "j"
Sleep 500ms
Type "l"
Sleep 1s
# Move to auth (skip api which is first child)
Type "j"
Sleep 500ms
Type "j"
Sleep 500ms
# Enter auth leaf to see tasks
Type "l"
Sleep 1s
# Enter the first task
Type "l"
Sleep 1s

Screenshot tui-task-detail.png
Type "q"
Sleep 500ms
