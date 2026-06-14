# harrypotter

A command line for the Harry Potter API.

`harrypotter` is a single pure-Go binary. It reads public Harry Potter data
from [hp-api.onrender.com](https://hp-api.onrender.com) over plain HTTPS,
shapes it into clean records, and prints output that pipes into the rest of
your tools. No API key, nothing to run alongside it.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
Harry Potter resources as `harrypotter://` URIs.

## Install

```bash
go install github.com/tamnd/harrypotter-cli/cmd/harrypotter@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/harrypotter-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/harrypotter:latest --help
```

## Usage

```bash
harrypotter characters                          # all 437 characters
harrypotter characters --house Gryffindor       # filter by house
harrypotter students                            # 103 Hogwarts students
harrypotter students --house Slytherin          # students in Slytherin
harrypotter staff                               # 25 Hogwarts staff members
harrypotter spells                              # all 77 spells
harrypotter --help                              # full command tree
```

Every command shares one output contract: `-o table|json|jsonl|csv|tsv|url|raw`,
`--fields` to pick columns, `--template` for a custom line, and `-n` to limit.
The default adapts to where output goes (a table on a terminal, JSONL in a
pipe), so the same command reads well by hand and parses cleanly downstream.

```bash
# pipe into jq
harrypotter characters -o json | jq '.[] | select(.alive)'

# get CSV of Slytherin students
harrypotter students --house Slytherin -o csv

# just names
harrypotter spells --fields name
```

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
harrypotter serve --addr :7777    # GET /v1/characters  returns NDJSON
harrypotter mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`harrypotter` registers a `harrypotter` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/harrypotter-cli/harrypotter"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `harrypotter://` URIs without knowing anything about the Harry Potter API:

```bash
ant get harrypotter://character/<uuid>   # fetch the character record
ant ls  harrypotter://characters         # list all characters
ant url harrypotter://spell/<uuid>       # the live https URL
```

## Development

```
cmd/harrypotter/   thin main: hands cli.NewApp to kit.Run
cli/               assembles the kit App from the harrypotter domain
harrypotter/       the library: HTTP client, data models, and domain.go (the driver)
docs/              tago documentation site
```

```bash
make build      # ./bin/harrypotter
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
