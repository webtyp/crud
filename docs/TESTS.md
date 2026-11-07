# Tests

| Test | Rule |
|---|---|
| [TestArgsJSON_ExactFormat](../tests/args_json_test.go) | Exact JSON wire serialization of CreateArgs, UpdateArgs, DeleteArgs |
| [TestArgsJSON_LoopbackDecode](../tests/args_json_test.go) | Roundtrip decode of args through loopback |
| [TestCaller_UserIDFromSession](../tests/caller_test.go) | Caller.UserID filled from authenticated session |
| [TestCaller_AnonymousLoopback](../tests/caller_test.go) | Caller.UserID empty on unauthenticated call |
| [TestRead_FilterDecodedAndValidated](../tests/filter_test.go) | Typed read filter decoded and validated against schema |
| [TestRead_FilterInvalidIs400](../tests/filter_test.go) | Invalid typed read filter returns 400 ErrInvalid |
| [TestRead_NilFilter](../tests/filter_test.go) | Module with nil newFilter receives nil filter in read callback |
| [TestOp_HasNoDotsForRPCCallerCompatibility](../tests/mount_test.go) | Operation names have no dots so `<module>.<op>` has exactly one dot |
| [TestCreate_RoundTripAndTransactionalBatch](../tests/mount_test.go) | Batch creation commits atomically; failure rolls back batch |
| [TestCreate_EmptyIs400](../tests/mount_test.go) | Create with no records returns 400 ErrNoRecords |
| [TestRead_RoundTripAndCapsLimit](../tests/mount_test.go) | ReadArgs caps limit to MaxPage |
| [TestDelete_RoundTrip](../tests/mount_test.go) | Delete ids round trip |
| [TestDelete_EmptyIs400](../tests/mount_test.go) | Delete with no ids returns 400 ErrNoIDs |
| [TestUpdate_RoundTrip](../tests/mount_test.go) | Update ids, fields and carrier record round trip |
| [TestUpdate_EmptyIs400](../tests/mount_test.go) | Update without ids or fields returns 400 |
| [TestUpdate_ErrorKeepsItsStatus](../tests/mount_test.go) | Domain errors preserve error kind (e.g. Conflict) |
| [TestCrudImportsNoUI](../tests/no_ui_test.go) | crud package never imports webtyp/dom or webtyp/view |
| [TestMount_PublicRead](../tests/public_test.go) | App-declared public reads mounted Public without Requires |
| [TestMount_NotPublicByDefault](../tests/public_test.go) | Reads default to requiring Resource(ModelName), Read |
| [TestMount_NilFunctionIsError](../tests/public_test.go) | Mount with nil function returns ErrNilFunction |
| [TestMount_RouteHasNoPublic](../tests/public_test.go) | crud.Route does not expose Public, Authenticated, or Requires |
| [TestCap](../tests/read_test.go) | Cap bounds input limit to 1..MaxPage |
| [TestReadArgs_RoundTrip](../tests/read_test.go) | ReadArgs travels wire both ways unchanged |
| [TestSecurity_BatchLimitExceeded](../tests/security_test.go) | Requests exceeding MaxPage are rejected with ErrBatchTooLarge |
| [TestSecurity_ImmutableAndUnknownFields](../tests/security_test.go) | Mutating id, tenant_id, unknown fields or duplicate fields is rejected |
| [TestSecurity_InvalidOrEmptyIDs](../tests/security_test.go) | Empty or whitespace-only IDs return ErrInvalidID |
| [TestSecurity_CreateSchemaValidation](../tests/security_test.go) | Schema constraints (e.g. Max length) enforced on create |
| [TestSecurity_SearchTermCapped](../tests/security_test.go) | Oversized search query rejected with ErrArgTooLong |
| [TestSecurity_DuplicateIDs_Allowed](../tests/security_test.go) | Duplicate IDs in Delete accepted |
| [TestSecurity_UpdateValueValidation](../tests/security_test.go) | Schema validation enforced on updated field values |
| [TestSecurity_CursorAndIDCapped](../tests/security_test.go) | Oversized cursor or ID rejected |
| [TestServerOnly_CreateRejected](../tests/server_only_test.go) | Creating record with ServerOnly field populated returns ErrServerOnlyField |
| [TestServerOnly_CreateEmptyAllowed](../tests/server_only_test.go) | ServerOnly NotNull field allowed empty on create |
| [TestServerOnly_UpdateRejected](../tests/server_only_test.go) | Updating ServerOnly field returns ErrServerOnlyField |
