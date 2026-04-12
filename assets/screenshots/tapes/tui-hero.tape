# Hero GIF: tree comes alive, culminating in the log stream modal
Output tui-hero.gif

Set Shell "bash"
Set Width 1200
Set Height 800
Set FontSize 16
Set FontFamily "Bitstream Vera Sans Mono"
Set Padding 20
Set Theme "Dracula"
Set WindowBar Colorful
Set TypingSpeed 50ms
Set PlaybackSpeed 1.0

# Boot the TUI and sync the alt-screen off-camera
Hide
Type "wolfcastle"
Enter
Sleep 10s
Type "R"
Sleep 2s
Type "R"
Sleep 1s
Show

# Brief hold on the collapsed root so the viewer gets their bearings
Sleep 700ms

# Expand warzone -> backend / frontend / infra appear
Type "l"
Sleep 800ms

# Step into backend and expand -> api, auth, database light up with state colors
Type "j"
Sleep 250ms
Type "l"
Sleep 900ms

# Drop to frontend and expand -> components, routing.
# Tree order after warzone+backend expanded:
#   warzone, backend, api, auth, database, frontend, infra, demo-app
# G lands on demo-app; two k's step up past infra to frontend.
Type "G"
Sleep 250ms
Type "k"
Sleep 150ms
Type "k"
Sleep 250ms
Type "l"
Sleep 900ms

# Climb back up to the auth leaf (the in_progress star of the show)
Type "g"
Sleep 200ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms
Type "j"
Sleep 150ms

# Enter auth to reveal its task rows
Type "l"
Sleep 900ms

# Focus the active task
Type "j"
Sleep 250ms
Type "l"
Sleep 1100ms

# Pop the log modal — the closing act, where the daemon's thinking shows through
Type "L"
Sleep 1800ms

# Scroll back through the recent LLM invocations and breadcrumbs
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms
Type "k"
Sleep 120ms

# Dwell on the model_invoke + breadcrumb run so it reads
Sleep 2500ms

Hide
Type "q"
Sleep 500ms
Show
