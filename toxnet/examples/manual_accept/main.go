// Package main demonstrates the correct friend-request flow with safety-number
// verification in toxnet.
//
// This example shows how to:
//  1. Create a ToxListener that does NOT auto-accept friend requests (the default).
//  2. Register a FriendRequestHandler that receives the peer's public key and a
//     precomputed safety number.
//  3. Display the safety number to the user for out-of-band verification before
//     calling tox.AddFriendByPublicKey.
//
// ⚠ SECURITY: Comparing safety numbers through a trusted out-of-band channel
// (e.g. a voice call or in-person meeting) is the only reliable way to detect
// man-in-the-middle attacks on the initial key exchange.
package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/opd-ai/toxcore"
	"github.com/opd-ai/toxcore/toxnet"
)

func main() {
	// ------------------------------------------------------------------
	// 1. Create a Tox instance for the server.
	// ------------------------------------------------------------------
	serverTox, err := toxcore.New(toxcore.NewOptions())
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create Tox instance: %v\n", err)
		os.Exit(1)
	}
	defer serverTox.Kill()

	fmt.Printf("Server Tox ID: %s\n", serverTox.SelfGetAddress())

	// ------------------------------------------------------------------
	// 2. Create a listener in manual-accept mode (the default).
	//    Listen() no longer auto-accepts friend requests; use ListenConfig
	//    with autoAccept=true only when MITM verification is not needed.
	// ------------------------------------------------------------------
	listener, err := toxnet.Listen(serverTox)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to create listener: %v\n", err)
		os.Exit(1)
	}
	defer listener.Close()

	tl := listener.(*toxnet.ToxListener)

	// ------------------------------------------------------------------
	// 3. Register the friend-request handler.
	//    The handler receives the peer's public key and a precomputed
	//    safety number.  Display it to the user and ask for confirmation
	//    before accepting.
	// ------------------------------------------------------------------
	tl.SetFriendRequestHandler(func(publicKey [32]byte, safetyNumber string) {
		fmt.Printf("\n─── Incoming friend request ───────────────────────────────\n")
		fmt.Printf("Peer public key : %x\n", publicKey)
		fmt.Printf("Safety number   : %s\n", safetyNumber)
		fmt.Printf("────────────────────────────────────────────────────────────\n")
		fmt.Println("Compare this safety number with the one shown on the")
		fmt.Println("peer's device through a trusted out-of-band channel")
		fmt.Println("(e.g. a voice call or in-person) before accepting.")
		fmt.Printf("Accept this friend request? [y/N]: ")

		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() && strings.EqualFold(strings.TrimSpace(scanner.Text()), "y") {
			if _, err := serverTox.AddFriendByPublicKey(publicKey); err != nil {
				fmt.Fprintf(os.Stderr, "AddFriendByPublicKey error: %v\n", err)
				return
			}
			fmt.Println("Friend request accepted.")
		} else {
			fmt.Println("Friend request rejected.")
		}
	})

	// ------------------------------------------------------------------
	// 4. Accept connections as usual once the friend-request is accepted.
	// ------------------------------------------------------------------
	fmt.Println("Listening for connections (Ctrl-C to quit)…")

	// Time out after 30 s in this demo so the binary does not run forever.
	done := time.After(30 * time.Second)
	go func() {
		<-done
		listener.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			fmt.Printf("listener closed: %v\n", err)
			return
		}
		fmt.Printf("Connection from %s\n", conn.RemoteAddr())
		conn.Close()
	}
}
