// Wolfcastle is Ralph on steroids. It's what happens when you give an
// action hero a task backlog and tell them not to come back until the
// job is done.
//
// Deterministic by design: state is JSON on disk and mutations go through
// compiled scripts with the soul of a 90s sysadmin but with none of the
// typos. Models only get called when something actually needs thinking.
//
// See https://github.com/dorkusprime/wolfcastle for documentation.
package main

import "github.com/dorkusprime/wolfcastle/cmd"

func main() {
	cmd.Execute()
}
