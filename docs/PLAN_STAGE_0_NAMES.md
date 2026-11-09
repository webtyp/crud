← [Plan](PLAN.md) | Next → [Stage 1](PLAN_STAGE_1_TESTS.md)

# Stage 0 — literal names (mechanical rename, no behaviour change)

Decision D47 and the naming rule of skill **api-design** («2 to 3 plain words; what does something carries a verb, what is something carries a concrete noun»). Every later stage of this plan writes the **new** names, so this
stage runs first and changes nothing else. Use the compiler: rename with `gopls rename` (or an editor rename), never by hand-typing a find-and-replace over comments and strings, and run `gotest` after each group.
File names are still the old ones in this stage (`table*.go`); stage 3 renames the files.

## 0.1 Functions and types (declared in the file shown)

| Old | New | Declared in |
|---|---|---|
| `createHandler`, `readHandler`, `updateHandler`, `deleteHandler` | `createRecordsHandler`, `readRecordsHandler`, `updateRecordsHandler`, `deleteRecordsHandler` | `table_create.go`, `table_read.go`, `table_update.go`, `table_delete.go` |
| `newLoopback` | `newLoopbackCaller` | `loopback.go` |
| `loopReg`, `loopOp`, `loopRoute`, `loopCaller`, `loopCtx`, `loopError`, `newLoopError` | `loopbackRegistry`, `loopbackOperation`, `loopbackRoute`, `loopbackCaller`, `loopbackContext`, `loopbackError`, `newLoopbackError` | `loopback.go` |
| `pkIndex`, `stampIndex` | `keyFieldIndexes`, `timestampFieldIndex` | `table.go` |
| `setString`, `setInt`, `atomicStore`, `keyOf`, `setKey`, `hasSeparator` | `setTextField`, `setIntField`, `inTransaction`, `keyValueOf`, `setKeyValue`, `keyHasSeparator` | `table.go` |
| `decodeStrings`, `createIn`, `copyRow` | `decodeTextList`, `createRequest`, `copyRecord` | `create.go`, `table_create.go`, `table_read.go` |
| `serverOnlyWritten` | `writesServerOnlyField` | `validate.go` |
| `registry` (type, in `mount.go`) | `operationMounter` | `mount.go` |
| `(r *registry) fail`, `isPublic` | `failMount`, `isPublicAction` | `mount.go` |
| `route`, `noRoute` | `mountedRoute`, `failedRoute` | `mount.go` |
| `callerOf` | `currentUserOf` | `mount.go` |

`mountError`, `validateIDs`, `validateReadArgs`, `validateCreateBatch`, `validateUpdateFields`, `statusNoContent` already say what they are and stay.

## 0.2 Parameters and locals inside the old `func (r *registry) Table(t Table)` (renamed to `AddStoredEntity` in stage 3)

`t` → `entity`, `schema` → `schemaFields`, `pk` → `keyFields`, `created` → `createdAtField`, `updated` → `updatedAtField` (the same words in the signatures of the four `…Handler` constructors above).
In the handlers: `in` → `request`, `skipped` → `alreadyStored`, `answers` → `ruleAnswers`.
Rename **every use inside the function body**, not only the parameter list (`schema` is used in the body of `readHandler` too): use `gopls rename` on the declaration so the compiler follows each use. The event publisher variable is called `publisher`, never `events`, which is the name of the imported package `webtyp.com/events`.

## 0.3 Standard library out (this package reaches the browser: `view` and `layout` import it)

`strings` is imported by `table.go`, `table_read.go`, `table_update.go`, `table_delete.go`, `validate.go` and `op.go`; the rule is `webtyp.com/fmt` (skill **wasm**, `AGENTS.md`). Replace each use with the `fmt` function of the same name:
`strings.Contains(a, b)` → `fmt.Contains(a, b)`, `strings.TrimSpace(s)` → `fmt.TrimSpace(s)`, `strings.ToLower(s)` → `fmt.ToLower(s)`, and swap the import `"strings"` for `"webtyp.com/fmt"`
(the code blocks of stage 3 already write `fmt`). `op.go` is deleted in stage 6, so only its uses are left behind until then.
Verify: `grep -rn '"strings"\|"errors"\|"strconv"' --include=*.go . | grep -v _test` → empty. Same commit.

## 0.4 Verify

`grep -rnw "createHandler\|readHandler\|updateHandler\|deleteHandler\|newLoopback\|loopReg\|loopOp\|loopRoute\|loopCaller\|loopCtx\|loopError\|newLoopError\|pkIndex\|stampIndex\|setString\|setInt\|atomicStore\|keyOf\|setKey\|hasSeparator\|decodeStrings\|createIn\|copyRow\|serverOnlyWritten\|callerOf\|noRoute\|isPublic" --include=*.go .` → empty.
`gotest` passes with the same tests and the same coverage as before (a pure rename). Commit: `refactor: literal names for crud's internal helpers`.
