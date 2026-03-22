# Log Command Rewrite

Replace the current follow.go implementation with a new wolfcastle log command that uses the logrender package. New flags: --follow/-f, --thoughts/-t, --interleaved/-i, --json, --session/-s. Remove the old --lines and --level flags. Default behavior: follow if daemon running, replay last session if stopped. Mutual exclusivity: --thoughts, --interleaved, and --json are mutually exclusive (last wins). Exit codes: 0=success, 1=no log files. The command wires session selection, output mode, and follow behavior together through the logrender package.
