package main

import (
	"fmt"

	"github.com/opd-ai/toxcore"
	avpkg "github.com/opd-ai/toxcore/av"
)

func main() {
	fmt.Println("ToxAV Call Control Demo")
	fmt.Println("=======================")
	fmt.Println()

	options := toxcore.NewOptions()
	tox, err := toxcore.New(options)
	if err != nil {
		panic(fmt.Sprintf("Failed to create Tox instance: %v", err))
	}
	defer tox.Kill()

	toxav, err := toxcore.NewToxAV(tox)
	if err != nil {
		panic(fmt.Sprintf("Failed to create ToxAV: %v", err))
	}
	defer toxav.Kill()

	fmt.Printf("ToxAV initialized successfully\n")
	fmt.Printf("Public Key: %X\n\n", tox.SelfGetPublicKey())

	friendNumber := uint32(1)

	fmt.Println("Available Call Control Commands:")
	fmt.Println("--------------------------------")
	fmt.Println()

	controls := []struct {
		name string
		ctrl avpkg.CallControl
		desc string
	}{
		{"Pause", avpkg.CallControlPause, "Stop media transmission temporarily"},
		{"Resume", avpkg.CallControlResume, "Resume media transmission"},
		{"Mute Audio", avpkg.CallControlMuteAudio, "Stop sending audio frames"},
		{"Unmute Audio", avpkg.CallControlUnmuteAudio, "Resume sending audio frames"},
		{"Hide Video", avpkg.CallControlHideVideo, "Stop sending video frames"},
		{"Show Video", avpkg.CallControlShowVideo, "Resume sending video frames"},
		{"Cancel", avpkg.CallControlCancel, "Terminate the call"},
	}

	for i, c := range controls {
		fmt.Printf("%d. %s - %s\n", i+1, c.name, c.desc)
		fmt.Printf("   toxav.CallControl(%d, av.CallControl%s)\n\n", friendNumber, c.ctrl.String())
	}

	fmt.Println("All call control commands are fully implemented!")
}
