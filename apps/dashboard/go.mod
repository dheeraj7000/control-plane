// This file exists solely to wall off apps/dashboard (a Next.js app,
// not Go code) from the root module's `go build ./...` / `go vet ./...`
// / `go test ./...` commands. Without it, those commands recurse into
// apps/dashboard/node_modules and pick up any stray .go files vendored
// inside an npm dependency (observed: flatted, a transitive dependency,
// ships one) as if they were part of this project. A directory
// containing its own go.mod is a separate module as far as the Go
// toolchain is concerned, so the parent module's `./...` pattern never
// descends into it — this module is otherwise unused, never built,
// and not listed as a dependency of the root module.
module github.com/dheeraj7000/control-plane/apps/dashboard/_gofence

go 1.26.5
