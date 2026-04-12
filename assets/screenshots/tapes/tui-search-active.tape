# Search screenshot: uses dedicated search staging directory
# Tree designed so "auth" matches:
#   warzone       olive (ancestor)
#   backend       olive (ancestor, expanded)
#     api         unhighlighted
#     auth        YELLOW (literal leaf match, expanded)
#       -> PKCE   YELLOW (literal task match, contains "auth")
#       -> Session unhighlighted task
#       -> Audit  unhighlighted
#     database    unhighlighted
#   frontend      olive (ancestor, COLLAPSED, contains auth-gateway)
#   infra         unhighlighted
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

# Navigate to auth and expand to show task rows
Type "g"
Sleep 500ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "j"
Sleep 200ms
Type "l"
Sleep 1s

# Search for "auth"
Type "/"
Sleep 500ms
Type "auth"
Enter
Sleep 1s

Screenshot tui-search-active.png
Escape
Sleep 500ms
Type "q"
Sleep 500ms
