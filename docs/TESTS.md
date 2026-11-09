# Tests

| Test | Rule |
|---|---|
| [TestCrudImportsNoUI](../tests/no_ui_test.go) | crud package never imports webtyp/dom or webtyp/view |
| [TestCap](../tests/read_test.go) | Cap bounds input limit to 1..MaxPage |
| [TestReadArgs_RoundTrip](../tests/read_test.go) | ReadArgs travels wire both ways unchanged |
| [TestReadArgs_EncodesOnlySetFields](../tests/read_test.go) | ReadArgs encodes only fields explicitly set |
| [TestRead_FilterDecodedAndValidated](../tests/filter_test.go) | Typed read filter decoded and validated against schema |
| [TestRead_FilterInvalidIs400](../tests/filter_test.go) | Invalid typed read filter returns 400 ErrInvalid |
| [TestRead_NilFilter](../tests/filter_test.go) | Module with nil newFilter receives nil filter in read callback |
| [TestStoredEntity_NothingAllowedFailsMount](../tests/stored_entity_test.go) | model with `Allow() == 0` → `crud.Mount` returns `ErrNothingAllowed` |
| [TestStoredEntity_NotAllowedActionDoesNotExist](../tests/stored_entity_test.go) | `Allow: Create|Read` → unmounted action cannot be reached |
| [TestStoredEntity_HookWithoutActionFailsMount](../tests/stored_entity_test.go) | hook provided for action not declared in Allow fails mount |
| [TestStoredEntity_CreateSetsIDAndStamps](../tests/stored_entity_test.go) | create without id → stored row has id, `created_at == updated_at > 0` |
| [TestStoredEntity_BatchOnNonTransactionalStoreRejected](../tests/stored_entity_test.go) | a Store without `Atomic`; create 2 records → `ErrBatchNotAtomic`; 1 record → stored |
| [TestStoredEntity_CreateBatchIsAtomic](../tests/stored_entity_test.go) | 2 records, hook fails on the 2nd → 0 rows stored |
| [TestStoredEntity_UpdateOnlyListedFields](../tests/stored_entity_test.go) | update fields `["name"]` → stored updated fields |
| [TestStoredEntity_UpdateMissingIsNotFound](../tests/stored_entity_test.go) | update id "nope" → error kind `model.ErrorNotFound` |
| [TestStoredEntity_UpdateHookSeesOldAndNext](../tests/stored_entity_test.go) | hook rejects when `old.name == "locked"` → row unchanged |
| [TestStoredEntity_SearchDefaultUsesItemLabel](../tests/stored_entity_test.go) | search default uses ItemLabel |
| [TestStoredEntity_SearchHookOrIsRejected](../tests/stored_entity_test.go) | `Search` returns `storage.Or(storage.Eq(...))` → `ErrOrCondition` |
| [TestStoredEntity_CompositeKeyRoundTrip](../tests/stored_entity_test.go) | composite key round trips properly |
| [TestStoredEntity_CompositeKeyPartRequired](../tests/stored_entity_test.go) | composite key parts are required |
| [TestStoredEntity_KeyWithSeparatorRejected](../tests/stored_entity_test.go) | key with separator is rejected |
| [TestStoredEntity_CompositeKeyWithInt](../tests/stored_entity_test.go) | composite key with integer value |
| [TestStoredEntity_PublicCreateWithoutSession](../tests/stored_entity_test.go) | mount operation handles public creation |
| [TestStoredEntity_ServerOnlyRejected](../tests/stored_entity_test.go) | server only fields are rejected |
| [TestMemStore](../tests/storetest_test.go) | in-memory store conforms to crud.Store and crud.Transactional contracts |
| [TestMount_DomainWithoutHookIsOutOfDate](../tests/mount_check_test.go) | AddMyBusinessRule without hook declared returns out of date error |
| [TestMount_HookWithoutDomainIsOutOfDate](../tests/mount_check_test.go) | HaveNoBusinessRule with hook declared returns out of date error |
| [TestMount_SearchHookFollowsReadRule](../tests/mount_check_test.go) | Search hook must match Read rule declaration |
| [TestMount_AllowedActionWithoutRuleIsRefused](../tests/mount_check_test.go) | allowed action without explicit rule mode is refused |
| [TestMount_HandlerListedButNotMounted](../tests/mount_check_test.go) | entity declared in Definition but unmounted is refused |
| [TestMount_MountedButNotListed](../tests/mount_check_test.go) | mounted entity missing from Definition is refused |
| [TestMount_DefinitionNameMustMatchModelName](../tests/mount_check_test.go) | module definition name must match ModelName |
| [TestMount_InternalHandlerIsNotMounted](../tests/mount_check_test.go) | internal entity with Allow == 0 does not require mounting |
| [TestMount_ConsistentModuleMounts](../tests/mount_check_test.go) | consistent module mounts operations successfully |
| [TestOperation_StoredEntityMountsOneOperationPerAllowedAction](../tests/op_test.go) | mounts exactly one operation per allowed action |
| [TestOperation_WireReadMountsItsOwnOperation](../tests/op_test.go) | wire read mounts qualified router.Operation |
| [TestOperation_ModuleIsPartOfTheIdentity](../tests/op_test.go) | module name isolates entities across modules |
| [TestOperation_UnknownOperationIsAnError](../tests/op_test.go) | unmounted operation invocation returns error |
| [TestWrite_AnnounceIsPublishedAfterCommit](../tests/write_flow_test.go) | event published only after transaction commit |
| [TestWrite_SilentStoresWithoutPublishing](../tests/write_flow_test.go) | ApplyWithoutPublishing stores row without publishing |
| [TestWrite_StandardAlwaysAnnounces](../tests/write_flow_test.go) | HaveNoBusinessRule action always announces event |
| [TestWrite_RuleErrorPublishesNothing](../tests/write_flow_test.go) | rule error rolls back transaction and publishes nothing |
| [TestWrite_ZeroResultIsRefused](../tests/write_flow_test.go) | zero RuleAnswer is rejected with ErrNoAnswer |
| [TestWrite_BatchRollbackPublishesNothing](../tests/write_flow_test.go) | failed batch rolls back and publishes nothing |
| [TestWrite_UpdateEventCarriesFieldsAndEachID](../tests/write_flow_test.go) | update event carries distinct record copy and touched fields |
| [TestWrite_UpdateSilentForOneID](../tests/write_flow_test.go) | individual record can suppress publishing in batch update |
| [TestWrite_DeleteEventCarriesStoredRow](../tests/write_flow_test.go) | delete event carries copy of stored record before deletion |
| [TestWrite_NoPublisherIsFine](../tests/write_flow_test.go) | write succeeds when events.Publisher is nil |
| [TestWrite_ReadNeverPublishes](../tests/write_flow_test.go) | read operations never publish events |
| [TestTopic_Shape](../tests/write_flow_test.go) | Topic formats module.entity.created/updated/deleted |
| [TestChange_EncodesRecordAndFields](../tests/write_flow_test.go) | Change encodes record object and fields array |
