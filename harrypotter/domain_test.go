package harrypotter

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network. The client's
// HTTP behaviour is covered in harrypotter_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "harrypotter" {
		t.Errorf("Scheme = %q, want harrypotter", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "harrypotter" {
		t.Errorf("Identity.Binary = %q, want harrypotter", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct {
		in  string
		typ string
		id  string
	}{
		{"9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8", "character", "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8"},
		{"https://hp-api.onrender.com/api/characters/9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8", "character", "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	id := "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8"
	got, err := Domain{}.Locate("character", id)
	want := "https://" + Host + "/api/characters/" + id
	if err != nil || got != want {
		t.Errorf("Locate = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocate_Spell(t *testing.T) {
	id := "a24082c1-3456-789a-bcde-f01234567890"
	got, err := Domain{}.Locate("spell", id)
	want := "https://" + Host + "/api/spells/" + id
	if err != nil || got != want {
		t.Errorf("Locate spell = (%q, %v), want (%q, nil)", got, err, want)
	}
}

func TestLocate_Unknown(t *testing.T) {
	_, err := Domain{}.Locate("unknown", "abc")
	if err == nil {
		t.Error("Locate with unknown type should return an error")
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a Character mints to its URI and resolves back correctly.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	ch := &Character{
		ID:   "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8",
		Name: "Harry Potter",
	}
	u, err := h.Mint(ch)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	want := "harrypotter://character/9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8"
	if u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("harrypotter", "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8")
	if err != nil || got.String() != want {
		t.Errorf("ResolveOn = (%q, %v), want %q", got.String(), err, want)
	}
}
