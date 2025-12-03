## Language Server Protocol Go Types

### Setup

`go get golang.org/x/tools/gopls/internal/lsp/protocol/generate@master`

### Generate

Generate 3 files: 
- Types: `tsprotocol.go` - Used as-is in `../protocol/types.go`
- JSON: `tsjson.go` - Used as-is in `../protocol/json.go`
- Server: `tsserver.go` - Uses Go's internal LS server, we use SourceGraph's. Can be used as inspiration for `../protocol/server.go`
- Client: `tsclient.go` - Not used

Steps:

- `go run golang.org/x/tools/gopls/internal/lsp/protocol/generate`
- `mv tsprotocol.go ../protocol/types.go`
- `mv tsjson.go ../protocol/json.go`
- `rm tsserver.go tsclient.go`
