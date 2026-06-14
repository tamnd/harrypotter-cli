package harrypotter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// sampleCharacters is a minimal set used by every character endpoint test.
var sampleCharacters = []wireCharacter{
	{
		ID:      "9e3f7ce4-b9a7-4244-b709-dae5c1f1d4a8",
		Name:    "Harry Potter",
		House:   "Gryffindor",
		Species: "human",
		Gender:  "male",
		Wizard:  true,
		Alive:   true,
		Wand:    struct {
			Wood   string  `json:"wood"`
			Core   string  `json:"core"`
			Length float64 `json:"length"`
		}{Wood: "holly", Core: "phoenix tail feather", Length: 11.0},
		Patronus:        "stag",
		HogwartsStudent: true,
		Actor:           "Daniel Radcliffe",
	},
	{
		ID:    "4c7b12e0-b6c8-4aa2-9b4c-1e2e3f4d5a6b",
		Name:  "Draco Malfoy",
		House: "Slytherin",
		Alive: true,
	},
}

var sampleSpells = []wireSpell{
	{ID: "a24082c1-3456-789a-bcde-f01234567890", Name: "Aberto", Description: "Opens locked doors"},
	{ID: "b35193d2-4567-89ab-cdef-012345678901", Name: "Accio", Description: "Summoning charm"},
}

func newTestServer(t *testing.T, handler http.HandlerFunc) (*httptest.Server, *Client) {
	t.Helper()
	srv := httptest.NewServer(handler)
	c := NewClient()
	c.Rate = 0 // no pacing in tests
	c.HTTP.Timeout = 5 * time.Second
	return srv, c
}

func jsonBody(v any) []byte {
	b, _ := json.Marshal(v)
	return b
}

func TestGet_UserAgent(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("User-Agent") == "" {
			t.Error("request carried no User-Agent")
		}
		_, _ = w.Write([]byte("ok"))
	})
	defer srv.Close()

	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "ok" {
		t.Errorf("body = %q, want %q", body, "ok")
	}
}

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	})
	defer srv.Close()
	c.Retries = 5

	start := time.Now()
	body, err := c.Get(context.Background(), srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestCharacters(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/characters" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write(jsonBody(sampleCharacters))
	})
	defer srv.Close()

	// point client at test server
	origBase := BaseURL
	_ = origBase // BaseURL is a const; we patch the URL via the method
	chars, err := c.fetchCharacters(context.Background(), srv.URL+"/api/characters", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 2 {
		t.Fatalf("got %d characters, want 2", len(chars))
	}
	hp := chars[0]
	if hp.Name != "Harry Potter" {
		t.Errorf("Name = %q, want Harry Potter", hp.Name)
	}
	if hp.WandWood != "holly" {
		t.Errorf("WandWood = %q, want holly", hp.WandWood)
	}
	if hp.WandCore != "phoenix tail feather" {
		t.Errorf("WandCore = %q, want phoenix tail feather", hp.WandCore)
	}
}

func TestCharacters_HouseFilter(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jsonBody(sampleCharacters))
	})
	defer srv.Close()

	chars, err := c.fetchCharacters(context.Background(), srv.URL+"/api/characters", "Gryffindor")
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 1 {
		t.Fatalf("got %d characters with Gryffindor filter, want 1", len(chars))
	}
	if chars[0].House != "Gryffindor" {
		t.Errorf("House = %q, want Gryffindor", chars[0].House)
	}
}

func TestCharacters_HouseFilterCaseInsensitive(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write(jsonBody(sampleCharacters))
	})
	defer srv.Close()

	chars, err := c.fetchCharacters(context.Background(), srv.URL+"/api/characters", "gryffindor")
	if err != nil {
		t.Fatal(err)
	}
	if len(chars) != 1 {
		t.Fatalf("house filter case-insensitive: got %d, want 1", len(chars))
	}
}

func TestSpells(t *testing.T) {
	srv, c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/spells" {
			t.Errorf("unexpected path %s", r.URL.Path)
		}
		_, _ = w.Write(jsonBody(sampleSpells))
	})
	defer srv.Close()

	// override the spells URL directly via a minimal wrapper
	body, err := c.Get(context.Background(), srv.URL+"/api/spells")
	if err != nil {
		t.Fatal(err)
	}
	var ws []wireSpell
	if err := json.Unmarshal(body, &ws); err != nil {
		t.Fatal(err)
	}
	if len(ws) != 2 {
		t.Fatalf("got %d spells, want 2", len(ws))
	}
	if ws[0].Name != "Aberto" {
		t.Errorf("Name = %q, want Aberto", ws[0].Name)
	}
}

func TestWireCharacter_toCharacter(t *testing.T) {
	wc := wireCharacter{
		ID:      "abc-123",
		Name:    "Hermione Granger",
		House:   "Gryffindor",
		Species: "human",
		Gender:  "female",
		Wizard:  true,
		Alive:   true,
		Wand: struct {
			Wood   string  `json:"wood"`
			Core   string  `json:"core"`
			Length float64 `json:"length"`
		}{Wood: "vine", Core: "dragon heartstring", Length: 10.75},
		Patronus:    "otter",
		Actor:       "Emma Watson",
		DateOfBirth: "19-09-1979",
		Ancestry:    "muggle-born",
	}

	ch := wc.toCharacter()
	if ch.ID != "abc-123" {
		t.Errorf("ID = %q, want abc-123", ch.ID)
	}
	if ch.WandWood != "vine" {
		t.Errorf("WandWood = %q, want vine", ch.WandWood)
	}
	if ch.WandCore != "dragon heartstring" {
		t.Errorf("WandCore = %q, want dragon heartstring", ch.WandCore)
	}
	if ch.Patronus != "otter" {
		t.Errorf("Patronus = %q, want otter", ch.Patronus)
	}
}
