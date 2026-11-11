---
PLAN: "feat: crud stores an uploaded file: Create with raw bytes, download route, no orphans"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 15655902573488726494
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST. Submit after EACH stage that compiles and keep committing.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> **Before you submit:** `git status` must be clean and `git diff --stat main` must list the files of the plan.

# Plan: file upload in `crud` (plan 4 of the upload design)

Decisions already taken (do not reopen): one file field per entity; `crud` declares the interface it needs and imports neither `input` nor `model` changes; the record keeps only the KEY of the file; the download uses the entity's `Read` permission; the client's `Content-Type` is never trusted.

## Ecosystem search
Reuse: `webtyp.com/files` (`files.ReadWriter`, `files.Remover`, `files.ErrNotExist`), `webtyp.com/filetype` (`filetype.Type{MIME, Ext}`), `router.QueryReader`. New: only the `crud` glue. `cloudflare/files.Store.Mount` (a separate raw PUT) is NOT touched here (plan 8 deletes it).

## Tools
```bash
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/files@v0.0.4 webtyp.com/filetype@v0.1.1
```
`webtyp.com/router` stays at the version `go.mod` pins (v0.5.0: it already has `Context.RawQuery()`).

## APIs used
| symbol | signature | file |
|---|---|---|
| `router.Context` | `Method() string; Path() string; RawQuery() string; Body() []byte; GetHeader(key string) string; SetHeader(key, value string); WriteStatus(code int); Write(b []byte) (int, error)` | `router/router.go:16` |
| `router.QueryReader` | `func QueryReader(query string) model.FieldReader` | `router/query_codec.go:58` |
| `model.Decodable` | `DecodeFields(r FieldReader)` on every record | `model/codec.go:66` |
| `files.Reader/Writer/Remover` | `ReadFile(path string) ([]byte, error)`, `WriteFile(path string, data []byte) error`, `RemoveFile(path string) error`; missing file → `files.ErrNotExist` (compare with `==`) | `files/files.go` |
| `filetype.Type` | struct with exported `MIME string`, `Ext string` | `filetype/filetype.go:20` |
| `router.WriteError(ctx, err)` | used in `stored_entity_create.go` | crud |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — types and mount check (`file_field.go`, new; `contract.go`; `stored_entity.go`)

```go
package crud

import (
	"webtyp.com/files"
	"webtyp.com/filetype"
	"webtyp.com/model"
)

// fileField is what crud needs from a field kind that holds an uploaded file; input.ImageFile & co. satisfy it.
type fileField interface {
	ValidateFile(data []byte) (filetype.Type, error)
	MaxBytes() int
}

// FileStore is the file store an entity with a file field writes to and reads from.
type FileStore interface {
	files.Reader
	files.Writer
	files.Remover
}

const (
	FileTypeSuffix = "_type" // optional text column next to the file field: the detected MIME
	FileSizeSuffix = "_size" // optional int64 column next to the file field: the size in bytes
	ErrNoFileStore    mountError = "crud: the entity has a file field but StoredEntity.Files is nil; pass the app's files store"
	ErrTwoFileFields  mountError = "crud: an entity can have only one file field; a second file is another table"
	ErrFileFieldNotText mountError = "crud: a file field must be a text column that holds the file key"
)

// fileFieldIndex returns the index of the entity's file field, or -1 when it has none.
func fileFieldIndex(schema []model.Field) (int, error) {
	found := -1
	for i, f := range schema {
		if _, ok := f.Type.(fileField); !ok {
			continue
		}
		if found >= 0 {
			return -1, ErrTwoFileFields
		}
		found = i
	}
	return found, nil
}
```
In `StoredEntity` (`contract.go`) add: `Files FileStore // nil unless the entity has a file field`. In `AddStoredEntity` (`stored_entity.go`), right after `keyFields` is computed: `fileIdx, err := fileFieldIndex(schemaFields)`; on error `r.failMount(err); return`; when `fileIdx >= 0` and `entity.Files == nil` → `r.failMount(ErrNoFileStore)`; when `ptrs[fileIdx]` is not `*string` → `ErrFileFieldNotText`. Read `Field.Type`'s Go type in `model/field.go` first (the field says `Type model.Kind`): if the assertion needs `f.Type` of another name, adapt only that and say so.

## Stage 2 — the upload path (`stored_entity_file.go`, new)

The Create route is the same operation; `createRecordsHandler` is replaced for file entities. In `AddStoredEntity` mount: `if fileIdx >= 0 { mountAction(model.Create, createFileHandler(entity, publisher, schemaFields, keyFields, fileIdx, createdAtField, updatedAtField, revisionField)) } else { …today's line… }`.

Body of the new handler (copy the single-record flow of `createRecordsHandler`; this is the exact difference):

```go
func createFileHandler(entity StoredEntity, publisher eventPublisher, schemaFields []model.Field, keyFields []int, fileIdx, createdAtField, updatedAtField, revisionField int) func(ctx router.Context) {
	kind := schemaFields[fileIdx].Type.(fileField)
	return func(ctx router.Context) {
		if n, ok := declaredLength(ctx.GetHeader("Content-Length")); ok && n > int64(kind.MaxBytes()) {
			router.WriteError(ctx, ErrFileTooLarge) // 413
			return
		}
		data := ctx.Body()
		detected, err := kind.ValidateFile(data) // bytes decide the type, never the client's Content-Type
		if err != nil {
			router.WriteError(ctx, ErrFileRejected) // 415
			return
		}
		rec := entity.NewList().Append().(model.Model)
		rec.DecodeFields(router.QueryReader(ctx.RawQuery())) // the record's own fields come in the query
		// … then exactly the single-record body of createRecordsHandler (key, stamps, entity.Create rule, enqueueCalls, tx.Create) with ONE addition
		// inside runWrite, before tx.Create(rec):
		//     key := entity.Store.NewID() + detected.Ext
		//     if err := entity.Files.WriteFile(key, data); err != nil { return err }
		//     written = key
		//     setTextField(rec, fileIdx, key); setFileMeta(rec, schemaFields, fileIdx, detected.MIME, len(data))
		// and when runWrite returns an error and written != "": _ = entity.Files.RemoveFile(written)  (no orphan)
	}
}
```
`declaredLength` parses the header with `webtyp.com/fmt` helpers if they exist (`grep -n "func.*Atoi\|ParseInt" ../fmt/*.go`), else a small loop of digits (no stdlib `strconv` if the package is imported by WASM; check `grep -rn '"strconv"' *.go`). `setFileMeta` sets `<name>_type` (`*string`) and `<name>_size` (`*int64`) **only if** a field with that exact name exists (`schemaFields[i].Name == schemaFields[fileIdx].Name+FileTypeSuffix`). Errors, both `model.Invalid` typed with a status: `ErrFileTooLarge = "crud: the file is larger than the field allows; send a smaller file"` → 413 and `ErrFileRejected = "crud: the file type is not accepted for this field; send one of the allowed types"` → 415. Find how crud maps an error to a status (`grep -rn "StatusCode\|WriteStatus" router/*.go | head`) and use the existing mechanism; never write a status by hand if one exists. The commit succeeds → response is the record, as today.

## Stage 3 — download (`stored_entity_file.go`)
Mount, for file entities only, one more operation: `r.reg.Operation(router.Operation{Module: r.module, Entity: modelName + ".file", Action: model.Read}, h)` with the same `isPublicAction`/`Requires(model.Resource(modelName), model.Read)` pair as the other actions (copy the lines of `mountAction`). Handler: id from `router.QueryReader(ctx.RawQuery())` key `id`; read the row through the entity's store the way `readRecordsHandler` reads one row (copy it), take the key field value, `data, err := entity.Files.ReadFile(key)`; `files.ErrNotExist` → not found. Headers: `Content-Type` = the stored `<name>_type` when the column exists, else derive from the key extension with a small lookup over `filetype` constants you can reach (`filetype.Detect(data)` is enough: use it); `X-Content-Type-Options: nosniff`; `Content-Disposition: inline`; `ctx.Write(data)`.

## Stage 4 — delete removes the file
In `deleteRecordsHandler`, for a file entity: after the transaction commits, `_ = entity.Files.RemoveFile(key)` for each deleted row (read the key before the delete). One line per path; nothing else changes.

## Stage 5 — tests (`tests/file_upload_test.go`, rows in `docs/TESTS.md`)
Use a test model with a text `logo` field of kind `input.ImageFile(1024)` (the `tests/` module already imports `input`? if not add `webtyp.com/input@v0.2.1` to `tests/go.mod`) plus `logo_type`/`logo_size`, `files/mem` as the store, `crud.LoopbackActingAs`-style mounting like the existing tests (read `tests/` for the helper that mounts a StoredEntity), a PNG made of its 8 signature bytes + padding, a PDF `%PDF-1.4` prefix.
- `TestFile_CreateStoresKeyTypeAndSize`: PNG → record has key ending `.png`, `logo_type` `image/png`, size = len; `files.mem` holds the bytes.
- `TestFile_RejectsWrongType`: PDF bytes to an image field → `ErrFileRejected`, nothing stored.
- `TestFile_RejectsTooLarge`: 2000 bytes, `MaxBytes` 1024 → `ErrFileTooLarge`.
- `TestFile_NoOrphanWhenRuleFails`: Create rule returns an error → `mem` is empty.
- `TestFile_DownloadReturnsBytesWithNosniff`: header `X-Content-Type-Options: nosniff`, bytes equal.
- `TestFile_DeleteRemovesBytes`.
- `TestMount_FileEntityNeedsStore`, `TestMount_TwoFileFieldsRefused`.
Each: setup → call → expected value named.

## Stage 6 — docs
`docs/ARCHITECTURE.md` gets a short section «Archivos» (Spanish is NOT required: `webtyp/*` is English): the 5 steps of the upload, the download, why the key is minted by the server. Add `docs/diagrams/FILE_UPLOAD.md` (context header, `flowchart TD` without `subgraph`, numbered walkthrough). Update the README feature list (one line).

## Delivery check
`gotest` passes on every destination; a tree that does not compile or whose tests did not run is a failed delivery. `git diff --stat main` lists `file_field.go`, `stored_entity_file.go`, `contract.go`, `stored_entity.go`, `stored_entity_delete.go`, the tests and docs.

| Stage | Files |
|---|---|
| 1 | `file_field.go`, `contract.go`, `stored_entity.go` |
| 2–3 | `stored_entity_file.go`, `stored_entity.go` |
| 4 | `stored_entity_delete.go` |
| 5 | `tests/file_upload_test.go`, `docs/TESTS.md` |
| 6 | `docs/ARCHITECTURE.md`, `docs/diagrams/FILE_UPLOAD.md`, `README.md` |
