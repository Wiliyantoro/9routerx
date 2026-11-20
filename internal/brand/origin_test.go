// 9RouterX — derived from the work of https://github.com/yanstd888/ — port and modifications by the maintainers — https://t.me/aiclub_id

package brand

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// The package is a passive provenance record: these tests pin the contract
// tools/scripts/detect-derivative.sh and a human reader rely on — the repo
// URL, the decoded attribution, and the marker fingerprint recomputed the
// same way from the embedded list.

func TestSourceRepo(t *testing.T) {
	want := "https://github.com/yanstd888/9RouterX"
	if got := SourceRepo(); got != want {
		t.Fatalf("SourceRepo() = %q, want %q", got, want)
	}
}

func TestAttributionDecodes(t *testing.T) {
	got := Attribution()
	if got == "" {
		t.Fatal("Attribution() is empty")
	}
	if got == attributionB64 {
		t.Fatal("Attribution() returned the wrapped base64 literal; decode failed")
	}
	if strings.ContainsAny(got, " \t\n") && strings.TrimSpace(got) != got {
		t.Fatalf("Attribution() is not trimmed: %q", got)
	}
	if !strings.Contains(got, "yanstd888") {
		t.Fatalf("Attribution() lost the account: %q", got)
	}
}

func TestMarkerHashMatchesEmbeddedList(t *testing.T) {
	sum := sha256.Sum256(markedFiles)
	if want := hex.EncodeToString(sum[:]); MarkerHash() != want {
		t.Fatalf("MarkerHash() != sha256 of embedded markerhash list")
	}
	if MarkerHash() != MarkerHash() {
		t.Fatal("MarkerHash() is not stable")
	}
}

func TestMarkedFilesCountsEntries(t *testing.T) {
	if MarkedFiles() <= 0 {
		t.Fatalf("MarkedFiles() = %d, want > 0", MarkedFiles())
	}
}

func TestGetAssemblesRecord(t *testing.T) {
	got := Get()
	if got.Repo != SourceRepo() {
		t.Fatalf("Get().Repo = %q, want %q", got.Repo, SourceRepo())
	}
	if got.Channel != "stable" {
		t.Fatalf("Get().Channel = %q, want %q", got.Channel, "stable")
	}
	if got.MarkerHash != MarkerHash() {
		t.Fatalf("Get().MarkerHash = %q, want %q", got.MarkerHash, MarkerHash())
	}
}
