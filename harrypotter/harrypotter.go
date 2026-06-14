// Package harrypotter is the library behind the harrypotter command line:
// the HTTP client, request shaping, and the typed data models for the
// Harry Potter API (hp-api.onrender.com).
//
// The Client here is the spine every command shares. It sets a real
// User-Agent, paces requests so a busy session stays polite, and retries the
// transient failures (429 and 5xx) that any public API throws under load.
// Build your endpoint calls and JSON decoding on top of it.
package harrypotter

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultUserAgent identifies the client to the Harry Potter API.
const DefaultUserAgent = "harrypotter-cli/0.1 (tamnd87@gmail.com)"

// Host is the site this client talks to.
const Host = "hp-api.onrender.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// Client talks to the Harry Potter API over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 20s timeout (onrender.com
// cold starts can be slow), a 500ms minimum gap between requests, and three
// retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 20 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      500 * time.Millisecond,
		Retries:   3,
	}
}

// Get fetches url and returns the response body. It paces and retries according
// to the client's settings. The caller owns nothing extra; the body is read
// fully and closed here.
func (c *Client) Get(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string) (body []byte, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, err
	}
	return b, false, nil
}

// pace blocks until at least Rate has passed since the previous request.
func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- output types ---

// Character holds the details for a single Harry Potter character.
type Character struct {
	ID          string `json:"id"           kit:"id"`
	Name        string `json:"name"`
	House       string `json:"house"`
	Species     string `json:"species"`
	Gender      string `json:"gender"`
	DateOfBirth string `json:"date_of_birth"`
	Ancestry    string `json:"ancestry"`
	Patronus    string `json:"patronus"`
	Actor       string `json:"actor"`
	Alive       bool   `json:"alive"`
	Wizard      bool   `json:"wizard"`
	WandWood    string `json:"wand_wood"`
	WandCore    string `json:"wand_core"`
}

// Spell holds the details for a single Harry Potter spell.
type Spell struct {
	ID          string `json:"id"          kit:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// --- wire types ---

// wireCharacter is the raw JSON shape from hp-api.onrender.com/api/characters.
type wireCharacter struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	AlternateNames []string `json:"alternate_names"`
	Species        string   `json:"species"`
	Gender         string   `json:"gender"`
	House          string   `json:"house"`
	DateOfBirth    string   `json:"dateOfBirth"`
	YearOfBirth    int      `json:"yearOfBirth"`
	Wizard         bool     `json:"wizard"`
	Ancestry       string   `json:"ancestry"`
	EyeColour      string   `json:"eyeColour"`
	HairColour     string   `json:"hairColour"`
	Wand           struct {
		Wood   string  `json:"wood"`
		Core   string  `json:"core"`
		Length float64 `json:"length"`
	} `json:"wand"`
	Patronus        string `json:"patronus"`
	HogwartsStudent bool   `json:"hogwartsStudent"`
	HogwartsStaff   bool   `json:"hogwartsStaff"`
	Actor           string `json:"actor"`
	Alive           bool   `json:"alive"`
	Image           string `json:"image"`
}

func (w wireCharacter) toCharacter() *Character {
	return &Character{
		ID:          w.ID,
		Name:        w.Name,
		House:       w.House,
		Species:     w.Species,
		Gender:      w.Gender,
		DateOfBirth: w.DateOfBirth,
		Ancestry:    w.Ancestry,
		Patronus:    w.Patronus,
		Actor:       w.Actor,
		Alive:       w.Alive,
		Wizard:      w.Wizard,
		WandWood:    w.Wand.Wood,
		WandCore:    w.Wand.Core,
	}
}

// wireSpell is the raw JSON shape from hp-api.onrender.com/api/spells.
type wireSpell struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
}

// --- client methods ---

// Characters returns all characters, optionally filtered by house (case-insensitive).
// An empty house string returns all characters.
func (c *Client) Characters(ctx context.Context, house string) ([]*Character, error) {
	return c.fetchCharacters(ctx, BaseURL+"/api/characters", house)
}

// Students returns all Hogwarts students, optionally filtered by house.
func (c *Client) Students(ctx context.Context, house string) ([]*Character, error) {
	return c.fetchCharacters(ctx, BaseURL+"/api/characters/students", house)
}

// Staff returns all Hogwarts staff members.
func (c *Client) Staff(ctx context.Context) ([]*Character, error) {
	return c.fetchCharacters(ctx, BaseURL+"/api/characters/staff", "")
}

// Spells returns all spells.
func (c *Client) Spells(ctx context.Context) ([]*Spell, error) {
	body, err := c.Get(ctx, BaseURL+"/api/spells")
	if err != nil {
		return nil, err
	}
	var ws []wireSpell
	if err := json.Unmarshal(body, &ws); err != nil {
		return nil, fmt.Errorf("parse spells: %w", err)
	}
	out := make([]*Spell, len(ws))
	for i, s := range ws {
		out[i] = &Spell{
			ID:          s.ID,
			Name:        s.Name,
			Description: s.Description,
		}
	}
	return out, nil
}

// fetchCharacters is the shared logic for fetching and optionally filtering characters.
func (c *Client) fetchCharacters(ctx context.Context, url, house string) ([]*Character, error) {
	body, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}
	var wcs []wireCharacter
	if err := json.Unmarshal(body, &wcs); err != nil {
		return nil, fmt.Errorf("parse characters: %w", err)
	}
	var out []*Character
	for _, wc := range wcs {
		ch := wc.toCharacter()
		if house != "" && !strings.EqualFold(ch.House, house) {
			continue
		}
		out = append(out, ch)
	}
	return out, nil
}
