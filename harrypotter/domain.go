package harrypotter

import (
	"context"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes the Harry Potter API as a kit Domain: a driver that a
// multi-domain host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/harrypotter-cli/harrypotter"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// harrypotter:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone harrypotter binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the harrypotter driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "harrypotter",
		Hosts:  []string{Host},
		Identity: kit.Identity{
			Binary: "harrypotter",
			Short:  "A command line for the Harry Potter API.",
			Long: `A command line for the Harry Potter API (hp-api.onrender.com).

harrypotter reads public Harry Potter data over plain HTTPS, shapes it into
clean records, and prints output that pipes into the rest of your tools. No API
key, nothing to run alongside it.`,
			Site: Host,
			Repo: "https://github.com/tamnd/harrypotter-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	// characters: list all characters, optionally filtered by house
	kit.Handle(app, kit.OpMeta{Name: "characters", Group: "read", List: true,
		Summary: "List all Harry Potter characters"}, listCharacters)

	// students: list Hogwarts students, optionally filtered by house
	kit.Handle(app, kit.OpMeta{Name: "students", Group: "read", List: true,
		Summary: "List Hogwarts students"}, listStudents)

	// staff: list Hogwarts staff
	kit.Handle(app, kit.OpMeta{Name: "staff", Group: "read", List: true,
		Summary: "List Hogwarts staff members"}, listStaff)

	// spells: list all spells
	kit.Handle(app, kit.OpMeta{Name: "spells", Group: "read", List: true,
		Summary: "List all Harry Potter spells"}, listSpells)

	// character: resolver op — fetch all characters and return the one matching the id
	kit.Handle(app, kit.OpMeta{Name: "character", Group: "read", Single: true,
		Summary: "Fetch a character by id", URIType: "character", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "character UUID"}}}, getCharacter)

	// spell: resolver op — fetch all spells and return the one matching the id
	kit.Handle(app, kit.OpMeta{Name: "spell", Group: "read", Single: true,
		Summary: "Fetch a spell by id", URIType: "spell", Resolver: true,
		Args: []kit.Arg{{Name: "id", Help: "spell UUID"}}}, getSpell)
}

// newClient builds the client from the host-resolved config, so a host and the
// standalone binary pace and identify themselves the same way.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type charactersInput struct {
	House  string  `kit:"flag" help:"filter by Hogwarts house"`
	Client *Client `kit:"inject"`
}

type studentsInput struct {
	House  string  `kit:"flag" help:"filter by Hogwarts house"`
	Client *Client `kit:"inject"`
}

type staffInput struct {
	Client *Client `kit:"inject"`
}

type spellsInput struct {
	Client *Client `kit:"inject"`
}

type characterInput struct {
	ID     string  `kit:"arg" help:"character UUID"`
	Client *Client `kit:"inject"`
}

type spellInput struct {
	ID     string  `kit:"arg" help:"spell UUID"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func listCharacters(ctx context.Context, in charactersInput, emit func(*Character) error) error {
	chars, err := in.Client.Characters(ctx, in.House)
	if err != nil {
		return mapErr(err)
	}
	for _, ch := range chars {
		if err := emit(ch); err != nil {
			return err
		}
	}
	return nil
}

func listStudents(ctx context.Context, in studentsInput, emit func(*Character) error) error {
	chars, err := in.Client.Students(ctx, in.House)
	if err != nil {
		return mapErr(err)
	}
	for _, ch := range chars {
		if err := emit(ch); err != nil {
			return err
		}
	}
	return nil
}

func listStaff(ctx context.Context, in staffInput, emit func(*Character) error) error {
	chars, err := in.Client.Staff(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, ch := range chars {
		if err := emit(ch); err != nil {
			return err
		}
	}
	return nil
}

func listSpells(ctx context.Context, in spellsInput, emit func(*Spell) error) error {
	spells, err := in.Client.Spells(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, s := range spells {
		if err := emit(s); err != nil {
			return err
		}
	}
	return nil
}

func getCharacter(ctx context.Context, in characterInput, emit func(*Character) error) error {
	chars, err := in.Client.Characters(ctx, "")
	if err != nil {
		return mapErr(err)
	}
	for _, ch := range chars {
		if strings.EqualFold(ch.ID, in.ID) {
			return emit(ch)
		}
	}
	return errs.NotFound("character %q not found", in.ID)
}

func getSpell(ctx context.Context, in spellInput, emit func(*Spell) error) error {
	spells, err := in.Client.Spells(ctx)
	if err != nil {
		return mapErr(err)
	}
	for _, s := range spells {
		if strings.EqualFold(s.ID, in.ID) {
			return emit(s)
		}
	}
	return errs.NotFound("spell %q not found", in.ID)
}

// --- Resolver: the URI-native string functions, pure and network-free ---

// Classify turns any accepted input into the canonical (type, id).
func (Domain) Classify(input string) (uriType, id string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", errs.Usage("unrecognized harrypotter reference: %q", input)
	}
	// strip a full https URL down to its last path segment (the UUID)
	if strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		parts := strings.Split(strings.TrimRight(input, "/"), "/")
		input = parts[len(parts)-1]
	}
	if input == "" {
		return "", "", errs.Usage("unrecognized harrypotter reference: %q", input)
	}
	return "character", input, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "character":
		return BaseURL + "/api/characters/" + id, nil
	case "spell":
		return BaseURL + "/api/spells/" + id, nil
	default:
		return "", errs.Usage("harrypotter has no resource type %q", uriType)
	}
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code.
func mapErr(err error) error {
	return err
}
