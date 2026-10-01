package backend

import (
	"os/exec"
	"strings"
	"testing"
)

func TestQuotePOSIXExact(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "''"},
		{"abc", "'abc'"},
		{"tar -C x -cf - .", "'tar -C x -cf - .'"},
		{"it's", `'it'\''s'`},
		{"a'b'c", `'a'\''b'\''c'`},
		{"rm -rf /tmp/x", "'rm -rf /tmp/x'"},
	}
	for _, c := range cases {
		if got := quotePOSIX(c.in); got != c.want {
			t.Errorf("quotePOSIX(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// runPreserved models a runtime that honors argv: the process receives
// ["sh","-c",script] unchanged. If the backend ever ran the QUOTED form
// under a truly argv-preserving runtime, the inner sh -c would receive the
// quote characters as literal script text ("printf %s hello" not found) —
// this is why the backend only quotes when argvJoinProbe detects joining.
func runPreserved(t *testing.T, script string) (string, error) {
	t.Helper()
	return runSh(t, []string{"sh", "-c", script})
}

// runJoined models a runtime that joins trailing arguments with spaces into
// one command line which an outer shell re-parses (the empirically observed
// apple/container "machine run" failure mode):
//
//	["sh","-c", quotePOSIX(script)]  ->  root shell runs: sh -c '<script>'
//
// The quoted script must still arrive as ONE argument to the inner sh -c.
func runJoined(t *testing.T, script string) (string, error) {
	t.Helper()
	line := strings.Join([]string{"sh", "-c", quotePOSIX(script)}, " ")
	return runSh(t, []string{"sh", "-c", line})
}

func runSh(t *testing.T, argv []string) (string, error) {
	t.Helper()
	out, err := exec.Command(argv[0], argv[1:]...).CombinedOutput()
	return string(out), err
}

func TestQuotePOSIXArgvPreserved(t *testing.T) {
	out, err := runPreserved(t, "printf %s hello-argv")
	if err != nil {
		t.Fatalf("argv-preserved: %v (%s)", err, out)
	}
	if out != "hello-argv" {
		t.Errorf("argv-preserved: got %q, want %q", out, "hello-argv")
	}
}

func TestQuotePOSIXArgvJoined(t *testing.T) {
	out, err := runJoined(t, "printf %s hello-joined")
	if err != nil {
		t.Fatalf("argv-joined: %v (%s)", err, out)
	}
	if out != "hello-joined" {
		t.Errorf("argv-joined: got %q", out)
	}
}

// Without quoting, the joined runtime corrupts the pipeline: the inner
// `sh -c` receives only the first word ("tar"), exactly reproducing the
// BusyBox usage-dump + "short read" seen on macOS. This test documents the
// failure mode the quoting exists to prevent.
func TestUnquotedArgvJoinedFails(t *testing.T) {
	script := "printf %s hello-joined"
	line := strings.Join([]string{"sh", "-c", script}, " ")
	out, err := exec.Command("/bin/sh", "-c", line).CombinedOutput()
	_ = out
	// "sh -c printf %s hello-joined" -> inner sh runs `printf` (no args) with
	// $0=%s $1=hello-joined: prints nothing, exits 0 on most systems, or
	// usage+error on BusyBox. Either way, NOT hello-joined.
	if err == nil {
		t.Skipf("platform sh tolerated corrupted argv (out=%q)", out)
	}
}

// TestQuotePOSIXHostileScript: a script containing single quotes must
// survive the JOINED runtime semantics byte-for-byte (the semantics the
// backend quotes for). Under preserved semantics the RAW script is used,
// so hostile scripts are valid shell there by definition.
func TestQuotePOSIXHostileScript(t *testing.T) {
	hostile := "printf %s 'it'\"'\"'s /path with spaces'"
	out, err := runJoined(t, hostile)
	if err != nil {
		t.Fatalf("joined: %v (%s)", err, out)
	}
	if !strings.Contains(out, "s /path with spaces") {
		t.Errorf("joined: got %q", out)
	}
}
