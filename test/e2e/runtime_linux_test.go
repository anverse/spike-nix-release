package e2e

import (
	"os"
	"strings"
	"syscall"
	"testing"
)

// TestSandboxRuntime reads the kernel log. gVisor answers with its own
// messages ("Starting gVisor..."); the test fails only when gVisor is
// expected and absent.
func TestSandboxRuntime(t *testing.T) {
	serviceURL(t, "SPIKED_URL")
	buf := make([]byte, 1<<16)
	n, err := syscall.Klogctl(3, buf)
	log := ""
	if err == nil {
		log = string(buf[:n])
	}
	gvisor := strings.Contains(log, "gVisor")
	t.Logf("gVisor kernel log: %t", gvisor)
	if os.Getenv("EXPECT_GVISOR") == "true" && !gvisor {
		t.Fatalf("expected to run under gVisor (klogctl err=%v)", err)
	}
}
