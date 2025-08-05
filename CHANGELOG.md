## v0.1.2 (2025-08-05)

### Fix

- **gha**: bump goreleaser action to support v2 config

## v0.1.1 (2025-08-05)

### Fix

- update .goreleaser.yaml to v2 format

## v0.1.0 (2025-08-05)

### Feat

- add GitHub Actions automated release workflow
- complete command validation and refactoring documentation
- add AWS resource validation package
- Add version, commit, and build date to version output
- First working params2env version

### Fix

- improve error handling in SSM parameter operations
- improve error handling in read command validation
- improve error handling in test cleanup
- prevent nil pointer dereference in config merging
- prevent path traversal and log injection in config loading
- improve code readability in delete_test.go
- prevent log injection in config loading (CWE-117)
- prevent path traversal in config loading (CWE-22,23)
- improve error handling across command modules
- **config**: prevent log injection in loadFile errors
- **config**: prevent log injection in validation errors
- **config**: prevent path traversal in home config loading
- **test**: secure file permissions for test config files
- **read**: secure file permissions for parameter output
- **ci**: prevent script injection in release workflow
- **config**: prevent log injection in config loading
- unit test issue in cmd/read.go
- readability and maintainability issues in the delete_test.go file
- resolve test failures and improve test reliability

### Refactor

- simplify KMS ARN construction in getReplicaKMSKeyID
- extract common test setup logic in config_test.go for better maintainability
- use more descriptive variable name for overwrite flag in ModifyParameter
- simplify prefix logic in formatEnvName for better readability
- extract common test setup logic in read_test.go for better maintainability
- extract common test logic in modify_test.go for better maintainability
- **validation**: simplify ValidateKMSKey function
- **config**: simplify mergeConfig function
- **test**: simplify mock SSM client tests
- **test**: split TestExecute into focused test functions
- **test**: extract duplicate flag setup in modify tests
- **test**: simplify runDeleteTest function signature
- **create**: improve readability of getReplicaKMSKeyID function
- **cmd**: improve read command documentation and organization
- **cmd**: improve modify command documentation and error handling
- **cmd**: improve delete command documentation and error handling
- **cmd**: improve create command documentation and organization
- **aws**: improve error handling and test coverage
- **config**: enhance configuration package with validation and documentation

### Perf

- avoid string allocation in logger level parsing
