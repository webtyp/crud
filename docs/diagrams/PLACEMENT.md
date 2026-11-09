# Where `webtyp/crud` sits in the framework

This diagram answers one question: who `crud` depends on, who depends on it, and in which binary each piece runs. It is drawn from the `go.mod` files and imports of the repos, not from memory;
when a repo starts or stops importing `crud`, this file changes in the same plan. No test walks it; the check is `grep -l 'webtyp.com/crud' */go.mod`.

```mermaid
flowchart TD
    MODEL["webtyp/model<br/>contracts only: Definition, Rules, Action, ModuleDefinition<br/>runs in: browser and server"]
    CHECK["webtyp/definitioncheck<br/>judges a declaration (Rules against Allow)<br/>runs in: browser and server"]
    ROUTER["webtyp/router<br/>contracts only: Operation, Caller, OperationRegistry<br/>runs in: browser and server"]
    STORAGE["webtyp/storage<br/>SQL contract; indexdb in the browser, sqlite or postgres on the server<br/>runs in: browser and server"]
    CRUD["webtyp/crud (the engine)<br/>decodes, validates, checks RBAC, runs the rule, stores, publishes<br/>runs in: browser (offline, demos) and server"]
    ORM["webtyp/orm<br/>implements crud.Store: TenantStore, GlobalStore"]
    GEN["webtyp/definitionc (build time only)<br/>writes definition_gen.go and the rule_*.go skeletons"]
    MOD["a domain module (veltylabs/modules/*)<br/>definition.go, rule_*.go, module.go"]
    TRANS["webtyp/rpc, webtyp/mcp, webtyp/dbsync<br/>call crud.Mount to offer the operations<br/>over HTTP, to an agent, from the offline queue"]
    UI["webtyp/view, webtyp/layout (browser)<br/>use crud.ReadArgs, crud.Module, crud.Loopback"]
    APP["webtyp/appkit and the app's config/<br/>chooses the database and injects it; the server main injects httpd"]

    CRUD -- "imports" --> MODEL
    CRUD -- "imports" --> CHECK
    CRUD -- "imports" --> ROUTER
    CRUD -- "imports" --> STORAGE
    ORM -- "imports, to implement Store" --> CRUD
    MOD -- "imports" --> CRUD
    GEN -. "reads definition.go, writes the module's code; imports no crud" .-> MOD
    TRANS -- "imports" --> CRUD
    UI -- "imports" --> CRUD
    APP -- "composes" --> TRANS
    APP -- "composes" --> ORM
    APP -- "composes" --> MOD
```

1. `crud` imports only contracts (`model`, `router`, `storage`) and the judge; it imports no database driver, no `httpd` and no UI.
2. `orm` imports `crud` to implement its `Store`; `crud` never imports `orm`, so the database is chosen by the app.
3. A domain module imports `crud` for the types its rules use (`CurrentUser`, `Tx`, `RuleAnswer`); `definitionc` writes the glue and never imports it.
4. `rpc`, `mcp` and `dbsync` are the three ways an operation reaches `crud`; each calls `crud.Mount` on a module and none re-implements validation.
5. `view` and `layout` run in the browser and use `crud` for the argument types and the in-process `Loopback`.
6. `appkit` and the app's `config/` are the only places that know which database, which transport and which modules exist together.
