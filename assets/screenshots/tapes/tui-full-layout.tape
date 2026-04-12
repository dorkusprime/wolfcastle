# Hero screenshot: fully expanded tree with task detail, daemon shows "hunting"
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
# Sync alt-screen with refresh
Type "R"
Sleep 2s
Type "R"
Sleep 1s

# Expand warzone
Type "l"
Sleep 1s

# Expand backend
Type "j"
Sleep 500ms
Type "l"
Sleep 1s

# Expand frontend. Tree order after warzone+backend expanded:
#   warzone, backend, api, auth, database, frontend, infra, demo-app
# G lands on demo-app, then k twice steps up past infra to frontend.
Type "G"
Sleep 500ms
Type "k"
Sleep 200ms
Type "k"
Sleep 500ms
Type "l"
Sleep 1s

# Navigate to auth leaf (go to top, count down)
Type "g"
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms

# Expand auth to show task rows
Type "l"
Sleep 1s

# Move to first task "Add OAuth2 PKCE flow"
Type "j"
Sleep 500ms

# Enter task detail
Type "l"
Sleep 1s

Screenshot tui-full-layout.png
Type "q"
Sleep 500ms
