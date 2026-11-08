# Tests

| Test | Rule |
|---|---|
| [TestCrudImportsNoUI](../tests/no_ui_test.go) | crud package never imports webtyp/dom or webtyp/view |
| [TestCap](../tests/read_test.go) | Cap bounds input limit to 1..MaxPage |
| [TestReadArgs_RoundTrip](../tests/read_test.go) | ReadArgs travels wire both ways unchanged |
| [TestRead_FilterDecodedAndValidated](../tests/filter_test.go) | Typed read filter decoded and validated against schema |
| [TestRead_FilterInvalidIs400](../tests/filter_test.go) | Invalid typed read filter returns 400 ErrInvalid |
| [TestRead_NilFilter](../tests/filter_test.go) | Module with nil newFilter receives nil filter in read callback |
| [TestTable_NothingAllowedFailsMount](../tests/table_test.go) | model with `Allow() == 0` → `crud.Mount` returns `ErrNothingAllowed` |
| [TestTable_NotAllowedActionDoesNotExist](../tests/table_test.go) | `Allow: Create|Read` → calling `_u` → error "unknown operation" |
| [TestTable_HookWithoutActionFailsMount](../tests/table_test.go) | `Update` hook with `Allow: Read` → `ErrHookWithoutAction` |
| [TestTable_CreateSetsIDAndStamps](../tests/table_test.go) | create without id → stored row has id, `created_at == updated_at > 0` |
| [TestTable_BatchOnNonTransactionalStoreRejected](../tests/table_test.go) | a Store without `Atomic`; create 2 records → `ErrBatchNotAtomic`; 1 record → stored |
| [TestTable_CreateBatchIsAtomic](../tests/table_test.go) | 2 records, hook fails on the 2nd → 0 rows stored |
| [TestTable_UpdateOnlyListedFields](../tests/table_test.go) | update fields `["name"]` → stored updated fields |
| [TestTable_UpdateMissingIsNotFound](../tests/table_test.go) | update id "nope" → error kind `model.ErrorNotFound` |
| [TestTable_UpdateHookSeesOldAndNext](../tests/table_test.go) | hook rejects when `old.name == "locked"` → row unchanged |
| [TestTable_CommittedAfterCommitOnly](../tests/table_test.go) | hook error → `Committed` never called; success → called once per record |
| [TestTable_CommittedUpdateCarriesEachID](../tests/table_test.go) | update ids `["a","b"]` → `Committed` sees id `a` then id `b` |
| [TestTable_SearchDefaultUsesItemLabel](../tests/table_test.go) | search default uses ItemLabel |
| [TestTable_SearchHookOrIsRejected](../tests/table_test.go) | `Search` returns `storage.Or(storage.Eq(...))` → `ErrOrCondition` |
| [TestTable_FilterEquality](../tests/table_test.go) | filter equality |
| [TestTable_CompositeKeyRoundTrip](../tests/table_test.go) | composite key round trips properly |
| [TestTable_CompositeKeyPartRequired](../tests/table_test.go) | composite key parts are required |
| [TestTable_KeyWithSeparatorRejected](../tests/table_test.go) | key with separator is rejected |
| [TestTable_CompositeKeyWithInt](../tests/table_test.go) | composite key with integer value |
| [TestTable_PublicCreateWithoutSession](../tests/table_test.go) | mount operation handles public creation |
| [TestTable_ServerOnlyRejected](../tests/table_test.go) | server only fields are rejected |
| [TestMemStore](../tests/storetest_test.go) | test |
