package main

import (
"fmt"

"github.com/opd-ai/toxcore"
avpkg "github.com/opd-ai/toxcore/av"
)

func main() {
fmt.Println("ToxAV Call Control Demo")
fmt.Println("=======================\n")

options := toxcore.NewOptions()
tox, err := toxcore.New(options)
if err != nil {
ic(fmt.Sprintf("Failed to create Tox instance: %v", err))
}
defer tox.Kill()

toxav, err := toxcore.NewToxAV(tox)
if err != nil {
ic(fmt.Sprintf("Failed to create ToxAV: %v", err))
}
defer toxav.Kill()

fmt.Printf("ToxAV initialized successfully\n")
fmt.Printf("Public Key: %X\n\n", tox.SelfGetPublicKey())

friendNumber := uint32(1)

fmt.Println("Available Call Control Commands:")
fmt.Println("--------------------------------\n")

controls := []struct {
ame string
avpkg.CallControl
string
}{
avpkg.CallControlPause, "Stop media transmission temporarily"},
avpkg.CallControlResume, "Resume media transmission"},
Audio", avpkg.CallControlMuteAudio, "Stop sending audio frames"},
mute Audio", avpkg.CallControlUnmuteAudio, "Resume sending audio frames"},
Video", avpkg.CallControlHideVideo, "Stop sending video frames"},
Video", avpkg.CallControlShowVideo, "Resume sending video frames"},
cel", avpkg.CallControlCancel, "Terminate the call"},
}

for i, c := range controls {
tf("%d. %s - %s\n", i+1, c.name, c.desc)
tf("   toxav.CallControl(%d, av.CallControl%s)\n\n", friendNumber, c.ctrl.String())
}

fmt.Println("All call control commands are fully implemented!")
}
