package main

import (
	"fmt"

	"github.com/opd-ai/toxcore"
)

func main() {
	fmt.Println("Testing AddTcpRelay implementation...")

	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		fmt.Printf("Failed to create Tox instance: %v\n", err)
		return
	}
	defer tox.Kill()

	err = tox.AddTcpRelay("test.relay.com", 3389, "F404ABAA1C99A9D37D61AB54898F56793E1DEF8BD46B1038B9D822E8460FAB67")
	if err != nil {
		fmt.Printf("Error: %v\n", err)
	} else {
		fmt.Println("Success: TCP relay added!")
	}
}
