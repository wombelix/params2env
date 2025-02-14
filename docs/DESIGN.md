<!--

SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0

-->

# Specification for Building the params2env CLI Tool

## 1. Overview

The params2env CLI tool is designed to interact with AWS SSM Parameter Store to:

* **Read** a parameter (and "export" it as an environment variable or write it to

  a file)

* **Create** a parameter

* **Modify** a parameter

It supports both command line flags and YAML configuration files with the following

precedence:

1. CLI arguments (highest priority)
1. Configuration file in the current directory (e.g. ./.params2env.yaml)
1. Configuration file in the home directory (e.g. ~/.params2env.yaml)

It also supports global flags such as --loglevel, --version, and --help.

This tool leverages the AWS SDK for Go v2 for AWS interactions and uses Go

standard libraries (flag, os, context, log/slog, testing) whenever possible. The

only external dependencies are for AWS and YAML parsing.

## 2. Project Structure & Folder Layout

Follow a modular design that separates CLI parsing, configuration management, AWS

SDK interactions, and business logic. Below is a recommended project layout:

```go

params2env/

├── cmd/

│   ├── root.go         // Global flags and subcommand dispatcher

│   ├── root_test.go    // Unit tests for the root package

│   ├── read.go         // Implements the "read" subcommand

│   ├── read_test.go    // Unit tests for the read subcommand

│   ├── create.go       // Implements the "create" subcommand

│   ├── create_test.go  // Unit tests for the create subcommand

│   ├── modify.go       // Implements the "modify" subcommand

│   └── modify_test.go  // Unit tests for the modify subcommand

├── config/

│   ├── config.go       // Loads and merges YAML configuration files

│   └── config_test.go  // Unit tests for configuration loading/merging

├── aws/

│   ├── ssm.go          // AWS SSM and STS role assumption wrappers

│   └── ssm_test.go     // Unit tests for AWS interactions

├── internal/

│   ├── logger.go       // Logging initialization and helper functions

│   └── logger_test.go  // Unit tests for logger setup (if applicable)

├── main.go             // Entry point: simply calls cmd.Execute()

├── go.mod

├── go.sum

└── README.md

```

### Benefits of This Structure

* **Discoverability:** Tests are located right next to the code they test, making

  it easier to find both.

* **Ease of Execution:** Running `go test ./...` in the root directory will

  automatically discover all tests.

* **Package Isolation:** Each package's tests are written within the same package

  context (or in a separate package if external black-box testing is desired).

### Additional Best Practices for Testing in Go

1. **Table-Driven Tests:** For functions with various input/output scenarios,
   consider using table-driven tests to cover multiple cases in a single test
   function
1. **Use Test Files to Validate Error Paths:** Ensure your tests cover error cases
   (for example, testing what happens if a configuration file is missing or
   invalid)
1. **Mock External Dependencies:** For packages like aws where you interact with
   external services, define interfaces and use test implementations (or mocks) to
   simulate AWS responses
1. **Keep Tests Fast and Deterministic:** Avoid dependencies on the environment
   (such as file system state) by using temporary directories (os.MkdirTemp) or
   in-memory mocks
1. **Coverage Tools:** Use the built-in Go coverage tools (go test -cover) to
   verify that your tests cover all branches of your code

By following this structure and these best practices, you'll ensure that your tests

are maintainable, discoverable, and in line with Go community standards.

**Notes:**

* Use **Go modules** (initialize with `go mod init git.sr.ht/~wombelix/params2env`)

* Place tests alongside source files when possible or in a dedicated `test`

  directory

* The `cmd` folder contains code that "wires up" the CLI subcommands

* The `config` package is responsible for loading configuration from YAML files

* The `aws` package wraps AWS SDK interactions to allow easier unit testing via

  interfaces

## 3. Dependencies & Best Practices

### 3.1. AWS SDK for Go v2

* Use the [AWS SDK for Go v2](https://aws.github.io/aws-sdk-go-v2/) to interact

  with SSM and STS

* **Best Practice:** Use context (with timeouts if needed) for AWS API calls

* **Tip:** When a role is provided, use the STS AssumeRole functionality. The SDK

  v2 provides helper packages (e.g. `stscreds`) to wrap role assumption

### 3.2. YAML Parsing

* Use a well-known YAML library such as [gopkg.in/yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)

  to parse configuration files

* **Best Practice:** Validate configuration after unmarshaling to ensure required

  fields are present

### 3.3. CLI Flag Parsing

* **Recommendation:** Use the standard library's `flag` package and a "subcommand"

  pattern

* **Alternative:** If you need a more robust solution, you might consider an

  external package like [spf13/cobra](https://github.com/spf13/cobra) but note

  that the design goal is to favor standard libraries

### 3.4. Logging

* Use the standard library's `log/slog` for logging

* **Best Practice:** Initialize logging in a dedicated module (`internal/logger.go`)

  and configure the log level based on the `--loglevel` flag

### 3.5. Unit Testing

* Use the standard library's `testing` package

* **Best Practice:** Aim for 100% code coverage by designing your code to be

  testable (inject dependencies via interfaces, avoid "global state", etc.)

* Consider table-driven tests especially for configuration merging and command-line

  flag parsing

## 4. Detailed Implementation Instructions

### 4.1. Main Entry Point

**File:** `main.go`

The `main.go` file should be minimal:

* Initialize logging

* Call the subcommand dispatcher (for example, `cmd.Execute()`)

**Example:**

```go

package main

import (

    "log/slog"

    "git.sr.ht/~wombelix/params2env/cmd"

)

func main() {

    logger := slog.New(slog.NewTextHandler(nil, nil))

    if err := cmd.Execute(); err != nil {

        logger.Error("Error executing command", "error", err)

        slog.Exit(1)

    }

}

```

### 4.2. Global Flags and Subcommand Dispatching

**File:** `cmd/root.go`

* Parse the global flags (e.g. `--loglevel`, `--version`, `--help`)

* Inspect `os.Args` to decide which subcommand to execute

* For example, after processing global flags, check that at least one more argument

  exists and switch based on its value ("read", "create", "modify")

**Example using `flag.FlagSet`:**

```go

package cmd

import (

    "flag"

    "fmt"

    "os"

)

var (

    logLevel string

    version  bool

)

func Execute() error {

    globalFlags := flag.NewFlagSet("params2env", flag.ExitOnError)

    globalFlags.StringVar(&logLevel, "loglevel", "info",

        "The log level: debug, info, warn, error, fatal, panic")

    globalFlags.BoolVar(&version, "version", false, "Print version and exit")

    if err := globalFlags.Parse(os.Args[1:]); err != nil {

        return err

    }

    if version {

        fmt.Println("params2env version 1.0.0")

        return nil

    }

    args := globalFlags.Args()

    if len(args) < 1 {

        printUsage()

        os.Exit(1)

    }

    subCmd := args[0]

    switch subCmd {

    case "read":

        return runRead(args[1:])

    case "create":

        return runCreate(args[1:])

    case "modify":

        return runModify(args[1:])

    default:

        fmt.Fprintf(os.Stderr, "Unknown subcommand: %s\n", subCmd)

        printUsage()

        os.Exit(1)

    }

    return nil

}

func printUsage() {

    fmt.Println(`Usage:

  params2env [global options] <subcommand> [subcommand options]

Global options:

  --loglevel  Log level (debug, info, warn, error, fatal, panic), default "info"

  --version   Print version and exit

  --help      Show help message

Subcommands:

  read       Read an SSM parameter and export as environment variable or file

  create     Create a new SSM parameter

  modify     Modify an existing SSM parameter

`)

}

```

### 4.3. Subcommand Implementation

For each subcommand, create a dedicated file in the `cmd` package.

#### 4.3.1. `read` Subcommand

**File:** `cmd/read.go`

**Required flags:**

* `--path` (the SSM parameter path)

* `--env` (the environment variable name)

**Optional flags:**

* `--region` (can also come from env var `AWS_REGION`)

* `--role` (for role assumption)

* `--output` (either `"env"` or `"file"`, default `"env"`)

* `--file` (file path, used only when `--output` is `"file"`)

* `--upper` (if environment variable name should be uppercase; default is true)

* `--env-prefix` (prefix to add to the variable name)

**Implementation steps:**

1. Create a new `flag.FlagSet` for the read subcommand
1. Define and parse its flags
1. Load and merge the configuration file if present
1. Merge CLI flags with configuration values
1. Validate required flags
1. Initialize AWS SDK
1. Call a function to retrieve the parameter value
1. Format the output according to `--output`
1. Log appropriate events

#### 4.3.2. `create` Subcommand

**File:** `cmd/create.go`

**Required flags:**

* `--path`

* `--value`

* `--type` (must be either `"String"` or `"SecureString"`)

**Optional flags:**

* `--region`

* `--replica`

* `--description`

* `--kms-id`

* `--role`

* `--overwrite` (default `"false"`)

**Implementation steps:**

1. Use a new `flag.FlagSet`
1. Validate required flags
1. Merge configuration values
1. Initialize AWS SSM client
1. Call a function that wraps the AWS SDK call
1. Log the result

#### 4.3.3. `modify` Subcommand

**File:** `cmd/modify.go`

**Required flags:**

* `--path`

* `--value`

**Optional flags:**

* `--region`

* `--replica`

* `--description`

* `--role`

**Implementation steps:**

Same as `create` but call an update/modify function in the AWS wrapper.

### 4.4. Configuration Management

**File:** `config/config.go`

Define a configuration struct that maps to the YAML structure:

```go

package config

import (

    "os"

    "path/filepath"

    "gopkg.in/yaml.v3"

)

type Config struct {

    Region    string        `yaml:"region,omitempty"`

    Replica   string        `yaml:"replica,omitempty"`

    Prefix    string        `yaml:"prefix,omitempty"`

    Output    string        `yaml:"output,omitempty"` // "env" or "file"

    File      string        `yaml:"file,omitempty"`

    Upper     *bool         `yaml:"upper,omitempty"`  // pointer to distinguish

    EnvPrefix string        `yaml:"env_prefix,omitempty"`

    Role      string        `yaml:"role,omitempty"`

    KmsKey    string        `yaml:"kms_key,omitempty"`

    Params    []ParamConfig `yaml:"params,omitempty"`

}

type ParamConfig struct {

    Name   string `yaml:"name"`

    Env    string `yaml:"env,omitempty"`

    Region string `yaml:"region,omitempty"`

    Output string `yaml:"output,omitempty"`

}

```

Implement a function `LoadConfig()` that:

1. Looks for a configuration file in the current directory
1. Looks for a configuration file in the user's home directory
1. Merges the two (with the current directory file taking precedence)
1. Returns a pointer to the `Config` struct or `nil` if no config is found

### 4.5. AWS SDK Wrappers

**File:** `aws/ssm.go`

Implement functions such as:

* `NewSSMClient(ctx, region, role string) (ssmClient, error)`

* `GetParameterValue(ctx, client, path string) (string, error)`

* `CreateParameter(...) error`

* `ModifyParameter(...) error`

### 4.6. Logging Initialization

**File:** `internal/logger.go`

Write an initialization function that sets up the global logger using `slog`:

```go

package internal

import (

    "log/slog"

    "os"

    "strings"

)

func InitLogger(level string) *slog.Logger {

    var logLevel slog.Level

    switch strings.ToLower(level) {

    case "debug":

        logLevel = slog.LevelDebug

    case "info":

        logLevel = slog.LevelInfo

    case "warn":

        logLevel = slog.LevelWarn

    case "error", "fatal", "panic":

        logLevel = slog.LevelError

    default:

        logLevel = slog.LevelInfo

    }

    handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel})

    return slog.New(handler)

}

```

### 4.7. Unit Testing & Best Practices

* Write unit tests for each package

* Use interfaces and inject mocks for AWS interactions

* Use table-driven tests for flag parsing and configuration merging

* Avoid side-effects in tests

* Test error paths thoroughly

## 5. Gaps & Improvements

### Identified Gaps

1. **Flag Handling:**
   * Clarify `--env` flag behavior
   * Document default values

1. **Configuration:**
   * Document merging logic
   * Add validation

1. **Error Handling:**
   * Define error responses
   * Add diagnostics
   * Handle I/O errors

1. **Security:**
   * Document IAM requirements
   * Handle credentials

### Suggested Improvements

1. **Features:**
   * Add batch processing
   * Add dry-run mode
   * Add verbose logging

1. **Documentation:**
   * Add detailed examples
   * Document configuration
   * Add troubleshooting guide

## 6. References

* **AWS SDK:**
   * [Documentation](https://aws.github.io/aws-sdk-go-v2/)
   * [Authorization](https://aws.github.io/aws-sdk-go-v2/docs/authorization/)

* **Go Packages:**
   * [yaml.v3](https://pkg.go.dev/gopkg.in/yaml.v3)
   * [flag](https://pkg.go.dev/flag)
   * [slog](https://pkg.go.dev/log/slog)

* **Best Practices:**
   * [Testing](https://golang.org/doc/effective_go.html#testing)
