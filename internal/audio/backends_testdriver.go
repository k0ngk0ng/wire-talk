//go:build talk_test_audio

package audio

import "github.com/gen2brain/malgo"

// Integration-test-only driver. Release builds must never use this build tag.
// See audio_test.go for the upstream enum offset in pinned malgo v0.11.24.
func nativeBackends() []malgo.Backend { return []malgo.Backend{malgo.Backend(14)} }
