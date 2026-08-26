package domain

import (
	"errors"
	"testing"
)

func TestErrorCodesPresent(t *testing.T) {
	want := []Code{
		CodeWallPanelMismatch, CodeSocketIncompatible, CodeStaleRuleDigest,
		CodeInvalidTopology, CodeMaterialImbalance, CodeFixedPointOverflow,
		CodeLeaseConflict, CodeLeaseExpired, CodeSlurryExpired, CodeVolumeOverclaim,
		CodePrefixViolation, CodePortSealOutOfOrder, CodeDeviceRetryPending,
		CodeGenerationConflict, CodeIdempotencyConflict, CodeReviewerConflict,
		CodeEvidenceIncomplete, CodeTerminalConflict, CodeConcurrentModification,
		CodePersistenceCorrupt,
	}
	seen := map[Code]bool{}
	for _, c := range want {
		if c == "" {
			t.Fatalf("empty code in list")
		}
		if seen[c] {
			t.Fatalf("duplicate code %q", c)
		}
		seen[c] = true
	}
	if len(want) != 20 {
		t.Fatalf("expected 20 stable codes, got %d", len(want))
	}
}

func TestErrorStringAndReasons(t *testing.T) {
	e := NewError(CodeWallPanelMismatch, "mismatch").WithReasons("a", "b")
	if e.Code != CodeWallPanelMismatch {
		t.Fatalf("unexpected code %q", e.Code)
	}
	if len(e.Reasons) != 2 {
		t.Fatalf("expected 2 reasons, got %d", len(e.Reasons))
	}
	if e.Error() == "" {
		t.Fatal("empty error string")
	}
}

func TestIsCode(t *testing.T) {
	e := NewError(CodeTerminalConflict, "lost race")
	if !IsCode(e, CodeTerminalConflict) {
		t.Fatal("expected IsCode true")
	}
	if IsCode(e, CodeLeaseExpired) {
		t.Fatal("expected IsCode false")
	}
	if IsCode(errors.New("plain"), CodeTerminalConflict) {
		t.Fatal("expected IsCode false for non-domain error")
	}
	if IsCode(nil, CodeTerminalConflict) {
		t.Fatal("expected IsCode false for nil")
	}
}
