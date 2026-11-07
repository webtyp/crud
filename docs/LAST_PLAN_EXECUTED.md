---
PLAN: "feat: crud — the wire contract of record operations (list paging, save, delete), moved out of view"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work.

# crud — list paging, save and delete contract

## Why

`webtyp.com/view` is the screen library (presenter, screens; it imports `webtyp.com/dom`). Since list paging, every
server decodes `view.ListArgs` and caps with `view.Cap`, so every server and edge Worker links the UI kit
(`edge → auth/authority → view → dom`, caught by `veltylabs/iam/tests/layering_test.go`). This repo takes the half of
`view` that both sides share — the wire contract — so a server imports `crud` and never `view`.

This plan only builds `crud`. Follow-up plans (not here): `view` deletes these files and imports `crud`; `auth`,
`access`, `layout`, `appkit` and the modules switch `view.ListArgs/Cap/MaxPage/MountSave/MountDelete` to `crud.*`.

## Ecosystem search

- `view` (`page.go`, `mount.go`, `payload.go`): the code being moved — **reuse** by moving, not copying.
- `router` (`OperationRegistry`, `Caller`, `loopback`): routing and calling — **reuse**; record shapes are not its concern.
- `rpc` (`WriteError`): HTTP transport and error mapping — **reuse**; record shapes are not transport.
- `model` (`Model`, `FieldWriter/Reader`, `Invalid`, `Definition`): **reuse**.
- No package named `crud`, `paging` or `list` exists in `webtyp/` or `veltylabs/`: **new**.

## API design gate

1. **Prior art** (unverified in this session): Google AIP-158 puts pagination (`page_size`, `page_token`) in the API
   contract and AIP-131…135 define List/Get/Create/Update/Delete without any UI; Django REST Framework keeps
   `rest_framework.pagination` apart from templates; Connect/Twirp messages are shared by client and server and know
   nothing about rendering. All three: the paging contract is API, not view.
2. **Names:** `crud.ListArgs`, `crud.Cap`, `crud.MaxPage`, `crud.MountSave`, `crud.MountDelete` keep the names
   consumers already use (only the package changes). New exported names: `crud.SaveArgs`, `crud.UpdateArgs`,
   `crud.DeleteArgs` — "the args of a save/update/delete call"; they replace `view`'s unexported `saveArgs`,
   `updateArgs`, `deleteArgs`, which must become visible because the browser encoder (in `view`) and the server
   decoder (here) must share one shape.
3. **Ledger:** concepts +1 package / −0; call-site lines 0 (import path only); ways to do the same thing 0 — `view`
   loses these symbols in its follow-up plan, no alias.
4. **Where:** one concern — what crosses the wire for record operations. No UI import (guard test below).
5. **Deletes:** the stub `crud.go`; later, in `view`: `page.go` (all but `Page`, `All`, `DefaultPage`, `declared`),
   `mount.go`, `payload.go`.

Note: `MountSave` keeps its type parameters exactly as in `view` today (it is moved, not redesigned); removing them is
out of scope.

## Source

The current code is copied under `_temp/view/` (this repo cannot see `webtyp/view`):
`_temp/view/page.go`, `_temp/view/mount.go`, `_temp/view/payload.go`, `_temp/view/tests/{mount,page,payload_json}_test.go`.

## Stage 1 — module and files

1. `go.mod`: `module webtyp.com/crud`, require `webtyp.com/model v0.2.13`, `webtyp.com/router v0.4.17`,
   `webtyp.com/rpc v0.2.15` (then `go mod tidy`). Nothing else from `webtyp.com/*`.
2. Delete `crud.go`.
3. `list.go` (package `crud`): from `_temp/view/page.go` copy **only** `MaxPage`, `Cap`, `ListArgs`, `ListArgsModel` and
   the five `ListArgs` methods. Do **not** copy `Page`, `All`, `DefaultPage`, `declared` (they stay in `view`). In every
   comment replace `view.` with `crud.`. `Cap`'s example becomes
   `rows, err := m.listPatients(m.tenantID, args.Search, args.After, crud.Cap(args.Limit))`. The `MaxPage` comment
   becomes: `// MaxPage is the most rows one list call returns; the server caps every request to it (Cap).`
4. `args.go`: the wire shapes, exported, with the JSON keys as unexported constants used by both encoder and decoder:

   ```go
   const (
   	keyRecords = "records"
   	keyIDs     = "ids"
   	keyFields  = "fields"
   	keyRecord  = "record"
   )

   // SaveArgs is what a screen sends to a save operation: whole records, saved in order.
   //
   // Example: caller.Call(op, &crud.SaveArgs{Records: recs}, saved, done)
   type SaveArgs struct{ Records []model.Model }

   // UpdateArgs is what a screen sends to patch fields: which rows, which columns, and a record carrying the values.
   //
   // Example: caller.Call(op, &crud.UpdateArgs{IDs: ids, Fields: fields, Record: rec}, nil, done)
   type UpdateArgs struct {
   	IDs    []string
   	Fields []string
   	Record model.Model
   }

   // DeleteArgs is what a screen sends to a delete operation: the ids to remove.
   //
   // Example: caller.Call(op, &crud.DeleteArgs{IDs: ids}, nil, done)
   type DeleteArgs struct{ IDs []string }
   ```

   Each gets `IsNil()` and `EncodeFields(w model.FieldWriter)` with the bodies of `saveArgs`, `updateArgs`,
   `deleteArgs` in `_temp/view/payload.go`, using the constants instead of the string literals.
   Copy `saveIn` and `deleteIn` (unexported, with their `IsNil` and `DecodeFields`) using the same constants.
5. `errors.go`:

   ```go
   const (
   	ErrNoRecords  model.Invalid = "crud: no records to save"
   	ErrNoIDs      model.Invalid = "crud: no ids given"
   	ErrCannotRead model.Invalid = "crud: cannot read the request"
   )
   ```
6. `mount.go`: copy `_temp/view/mount.go` (`statusNoContent`, `MountSave`, `MountDelete`), package `crud`, examples
   say `crud.MountSave` / `crud.MountDelete`.

## Stage 2 — tests (`tests/`, package `tests`)

1. `tests/mount_test.go`: port `_temp/view/tests/mount_test.go`. It must not import `view`: replace the
   `view.NewRecords().Caller(...)` + `view.NewCallerLister(...)` round trip with the loopback caller directly —
   `c := loopback.ActingAs("t", "u", mod)` then `c.Call("save", &crud.SaveArgs{Records: []model.Model{rec}}, out, done)`
   and `c.Call("delete", &crud.DeleteArgs{IDs: ids}, nil, done)`. Keep every assertion (saved id filled, ids received
   in order, `ErrNoRecords`/`ErrNoIDs` on empty input). Declare a tiny test model in the test file instead of
   `view/conformance.MockRecord`.
2. `tests/list_test.go`: port `TestCap` and `TestListArgs_RoundTrip` from `_temp/view/tests/page_test.go`
   (`TestCallerLister_SendsSearchAndPage` stays in `view`).
3. `tests/args_json_test.go`: for each of `SaveArgs`, `UpdateArgs`, `DeleteArgs` encode with `webtyp.com/json` and
   assert the exact JSON (`{"records":[…]}`, `{"ids":[…],"fields":[…],"record":{…}}`, `{"ids":[…]}`), and that
   `saveIn`/`deleteIn` decode what `SaveArgs`/`DeleteArgs` encode (through `MountSave`/`MountDelete` with loopback).
4. `tests/no_ui_test.go` — the guard:

   ```go
   func TestCrudImportsNoUI(t *testing.T) {
   	cmd := exec.Command("go", "list", "-deps", "webtyp.com/crud")
   	cmd.Env = append(os.Environ(), "GOOS=js", "GOARCH=wasm")
   	out, err := cmd.CombinedOutput()
   	if err != nil {
   		t.Fatalf("go list: %v\n%s", err, out)
   	}
   	for _, f := range []string{"webtyp.com/dom", "webtyp.com/html", "webtyp.com/form", "webtyp.com/layout", "webtyp.com/svg", "webtyp.com/view"} {
   		for _, line := range strings.Split(string(out), "\n") {
   			if line == f {
   				t.Fatalf("crud must not import UI package %s: servers and edge Workers import crud", f)
   			}
   		}
   	}
   }
   ```

## Stage 3 — docs

1. `README.md`: one-line description (as `go.mod`'s repo description), then an «I want to… → use» table:
   decode a list call → `var args crud.ListArgs; ctx.Decode(&args)` + `.Accepts(&crud.ListArgs{})`; cap the limit →
   `crud.Cap(args.Limit)`; mount save → `crud.MountSave(reg, op, save).Requires(...)`; mount delete →
   `crud.MountDelete(reg, op, remove).Requires(...)`; send a save/update/delete from a client →
   `caller.Call(op, &crud.SaveArgs{…}, …)`. One sentence: «servers import `crud`, never `webtyp.com/view`».
2. Delete `_temp/`.

## Delivery check

```bash
gotest                                   # green, includes TestCrudImportsNoUI
test ! -e _temp && test ! -e crud.go
GOOS=js GOARCH=wasm go list -deps . | grep -x 'webtyp.com/dom' && exit 1 || true
git diff --stat main                     # go.mod, go.sum, list.go, args.go, errors.go, mount.go, tests/*, README.md, AGENTS.md; crud.go and _temp deleted
```
