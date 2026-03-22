// main.go is the entry point. In Go, every executable must have package main + func main().
// This is like the "bin" field in package.json — it's what runs when you type `migratex`.
package main

import "github.com/vswaroop04/migratex/cmd"

func main() {
	cmd.Execute()
}
