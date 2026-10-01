package backend

import "strings"

// quotePOSIX quotes s as exactly one POSIX shell word.
//
// Background: apple/container's "container machine run" does not
// deterministically preserve argv. Depending on version it may join the
// trailing arguments into a single command line that a shell then re-parses
// (docker-exec style). If ws passes ["sh","-c","tar -C a -cf - . | ..."] as
// three separate arguments, a joining runtime executes:
//
//	sh -c tar -C a -cf - . | tar ...
//
// i.e. the inner `sh -c` receives only the word "tar", and the pipeline
// runs in the outer shell with a broken first stage.
//
// Embedding the script via quotePOSIX:
//
//	["sh","-c", quotePOSIX(script)]
//
// is correct under BOTH semantics:
//   - argv preserved:   sh -c '<script>'   (script as one word)
//   - argv joined:      sh -c 'quote(script) ...' re-parsed by the outer
//     shell yields the same single-word script argument
//
// Uses the standard '\” escaping understood by dash, bash, and BusyBox ash.
func quotePOSIX(s string) string {
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}
