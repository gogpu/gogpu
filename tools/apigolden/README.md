# apigolden — the public-API golden of gogpu

`apigolden` prints every exported declaration of the module's public packages — types, fields, interface
method sets, functions, methods, constants and variables, with full signatures — as deterministic text.
The committed files under `testdata/` are the frozen public shape; a change to them is a change to a
contract consumers depend on.

The public set is eight packages: `gogpu`, `gmath`, `golden`, `gpu`, `gpu/backend/native`, `gpu/types`,
`input`, `sound`. Internals, examples, tools and tmp are excluded by construction; a new top-level
package must be added to `publicPackages` deliberately.

## Regenerating and checking

```bash
GOWORK=off go -C tools run ./apigolden -out apigolden/testdata/api_windows.txt
GOWORK=off go -C tools run ./apigolden -goos linux  -out apigolden/testdata/api_linux.txt
GOWORK=off go -C tools run ./apigolden -goos darwin -goarch arm64 -out apigolden/testdata/api_darwin.txt
GOWORK=off go -C tools run ./apigolden -goos js -goarch wasm -out apigolden/testdata/api_wasm.txt
git diff --exit-code tools/apigolden/testdata/   # must be empty
```

## Why four contexts

gogpu ships platform-specific implementations behind one public API, so each context is generated and
committed separately: `windows/amd64`, `linux/amd64`, `darwin/arm64`, `js/wasm`. The four goldens differ
where the shipped API really differs — for example `BackendInfo`'s parameter is named on native and `_`
on wasm — and that difference is part of the record. A context that is not declared in `contextFloors`
is refused rather than given a weak default.

## The floor

`contextFloors` pins the package and identifier counts the loader must reach before a golden is written
(currently 8 packages / 325 identifiers in every context). A load that loses a package or drops
identifiers fails instead of producing a short file that looks reviewed. When an intended public-API
change moves the count, regenerating includes lowering the floor to the reviewed count in the same
commit.

## Regenerating after an intended change

1. Edit the code.
2. Regenerate all four contexts.
3. Review the diff as a public-API change: removing or narrowing an exported name is a breaking change
   and needs the ecosystem API-CHANGE round before it ships.
4. Commit the code and the regenerated goldens together.

The formatter is covered by unit tests (`GOWORK=off go -C tools test ./apigolden`) that build packages in
memory, so they cannot be affected by the tree under analysis.
