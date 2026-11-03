package problem

// Versions of the problem protocol (docs/protocol.md, "Protocol versions").
// A pack declares the version it is written against in pack.yaml, and hc
// reads a pack only when MinProtocol <= protocol <= Protocol.
const (
	// Protocol is the newest protocol version hc implements. While it is a
	// draft (newer than FinalProtocol), protocol changes go into it. Once it
	// is final, an additive change, such as a new type, compare mode or
	// field, bumps Protocol only: packs written against an older version
	// stay readable, and the spec marks the new feature with the version
	// that introduced it.
	Protocol = 1
	// FinalProtocol is the newest final protocol version, 0 while none is.
	// A version becomes final with the first stable (not pre-release) hc
	// release that implements it, and never changes after that: the release
	// sets FinalProtocol and snapshots the version's features, which
	// TestFinalProtocol (internal/regression) then holds it to.
	FinalProtocol = 0
	// MinProtocol is the oldest protocol version hc still reads. An
	// incompatible change, one that alters or removes what an existing
	// version means, bumps Protocol and raises MinProtocol to it, so that hc
	// refuses older packs with a message telling their authors to update
	// them instead of misreading them.
	MinProtocol = 1
)

// Language ids of the protocol, as used in pack.yaml's languages and for
// solution directories.
const (
	LangGo     = "go"
	LangPython = "python"
)

// Languages returns the solution languages of the given protocol version, in
// a fixed order, or nil for a version hc does not implement. Protocol 1 has
// Go and Python.
func Languages(protocol int) []string {
	if protocol < MinProtocol || protocol > Protocol {
		return nil
	}
	return []string{LangGo, LangPython}
}
