<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: Apache-2.0
-->

# AWS SSM Parameter Store to Environment variables

[![REUSE status](https://api.reuse.software/badge/git.sr.ht/~wombelix/params2env)](https://api.reuse.software/info/git.sr.ht/~wombelix/params2env)
[![builds.sr.ht status](https://builds.sr.ht/~wombelix/params2env.svg)](https://builds.sr.ht/~wombelix/params2env?)
[![Release](https://github.com/wombelix/params2env/actions/workflows/release.yml/badge.svg)](https://github.com/wombelix/params2env/actions/workflows/release.yml)

## Table of Contents

* [Installation](#installation)
   * [GitHub Action](#github-action)
   * [Go Install](#go-install)
* [Shell Completion](#shell-completion)
* [CLI](#cli)
   * [Technical details](#technical-details)
* [Usage](#usage)
   * [Subcommand: read](#subcommand-read)
   * [Subcommand: create](#subcommand-create)
   * [Subcommand: modify](#subcommand-modify)
   * [Subcommand: delete](#subcommand-delete)
   * [YAML configuration file reference](#yaml-configuration-file-reference)
* [Build and Test](#build-and-test)
   * [Makefile](#makefile)
   * [Integration Tests](#integration-tests)
* [Source](#source)
* [Contribute](#contribute)
* [License](#license)

## CLI

`params2env` reads AWS SSM Parameter Store parameters and writes them
as environment variables. Also supports create, modify, and delete.

### Technical details

Uses AWS Go SDK. Minimal dependencies, prefers Go standard library
(slog, testing) where possible.

Config via YAML file (`~/.params2env.yaml` or `.params2env.yaml`) or
CLI args. CLI args override config. Local config takes precedence
over home directory.

## Installation

Binaries for Linux, macOS, and Windows on
[GitHub Releases](https://github.com/wombelix/params2env/releases).
Download and add to PATH.

### GitHub Action

Use the reusable action to install params2env in your workflows (Linux runners only):

```yaml
- uses: wombelix/params2env@v0.5.0
```

This installs the latest released version of the CLI. To pin a specific CLI version:

```yaml
- uses: wombelix/params2env@v0.5.0
  with:
    version: 0.4.0
```

The action version (e.g., `@v0.5.0`) and CLI version can differ,
allowing you to use newer action while pinning the CLI.

### Go Install

Or install via Go:

```bash
go install git.sr.ht/~wombelix/params2env@latest
```

## Shell Completion

Generate shell completion scripts for tab completion of commands and flags.

```bash
# Bash (add to ~/.bashrc)
source <(params2env completion bash)

# Zsh (add to ~/.zshrc)
source <(params2env completion zsh)

# Fish (add to ~/.config/fish/config.fish)
params2env completion fish | source

# PowerShell (add to $PROFILE)
params2env completion powershell | Out-String | Invoke-Expression
```

## Usage

Global flags:

* `--loglevel`: `debug`, `info`, `warn`, `error`, `fatal`, `panic` (default: `info`)
* `--version`: Print version
* `--help`: Print help

### Subcommand: read

* `--region`: AWS region (or use `AWS_REGION`/`AWS_PROFILE` env vars)
* `--path`: Parameter path (required)
* `--role`: IAM role ARN to assume
* `--file`: Output file (default: stdout)
* `--format`: Output format: `env` or `github-env` (default: `env`)
* `--upper`: Uppercase env var names (default: `true`)
* `--env-prefix`: Prefix for env var names
* `--env`: Custom env var name (overrides auto-generated name)

**Environment variable naming:**

By default, the env var name is the last segment of the parameter path.
With `--upper` (enabled by default), it gets uppercased.

| Parameter Path | `--env-prefix` | `--upper` | Result |
|----------------|----------------|-----------|--------|
| `/app/db_password` | - | true | `DB_PASSWORD` |
| `/app/db_password` | `APP` | true | `APP_DB_PASSWORD` |
| `/app/db_password` | - | false | `db_password` |

Use `--env` to set a fully custom name when auto-generation doesn't fit.

**Output formats:**

* `env` (default): `export KEY="value"` - for shell sourcing
* `github-env`: `KEY=value` - for GitHub Actions with automatic masking

**GitHub Actions usage:**

The `github-env` format automatically:

* Outputs masking commands (`::add-mask::value`) to stdout
* Append `KEY=value` format to `$GITHUB_ENV` file

```bash
# GitHub Actions workflow
- name: Load secrets
  run: |
    params2env read --path "/app/db-password" --format github-env
    # Automatically writes to $GITHUB_ENV and masks the value

# Manual file specification
params2env read --path "/app/secret" --format github-env --file secrets.env
```

Example:

```bash
params2env read --region "eu-central-1" --path "/my/secret" \
  --role "arn:aws:iam::111122223333:role/my-role" \
  --file "~/.my-secret" --upper "false" \
  --env-prefix "my_" --env "secret"

# GitHub Actions format
params2env read --path "/app/db-password" --format github-env

# Traditional shell format
params2env read --path "/app/config" --format env
```

Result in `~/.my-secret`:

```bash
export my_secret="<secret-value>"
```

To set env vars in your shell:

```bash
# Using eval
eval $(params2env read --path "/my/secret")

# Using source
source <(params2env read --path "/my/secret")
```

### Subcommand: create

* `--region`: AWS region (or use `AWS_REGION` env var)
* `--replica`: Replica region
* `--path`: Parameter path (required)
* `--description`: Parameter description
* `--value`: Parameter value (optional, see below)
* `--type`: `String` or `SecureString` (default: `String`)
* `--kms`: KMS Key ID for SecureString (e.g., `alias/aws/ssm` or `alias/myapp-key`)
* `--role`: IAM role ARN to assume
* `--overwrite`: Overwrite existing (default: `false`)

**Value input methods (in order of precedence):**

1. `--value` flag
1. Piped stdin (e.g., `echo "secret" | params2env create ...`)
1. Interactive prompt (SecureString: hidden, String: visible)

Example:

```bash
params2env create --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --description "Secret stored as SecureString" \
  --value "S3cr3t" --type "SecureString" \
  --kms "alias/myapp-key" \
  --role "arn:aws:iam::111122223333:role/my-role"

# Pipe value to avoid shell history
echo "S3cr3t" | params2env create --path "/my/secret" \
  --type "SecureString" --kms "alias/myapp-key"

# AWS managed key
params2env create --path "/my/secret" --type "SecureString" --kms "alias/aws/ssm"

# Interactive (prompts for value, hidden for SecureString)
params2env create --path "/my/secret" --type "SecureString" --kms "alias/myapp-key"
```

### Subcommand: modify

* `--region`: AWS region (or use `AWS_REGION` env var)
* `--replica`: Replica region
* `--path`: Parameter path (required)
* `--description`: Parameter description
* `--value`: New value (optional, see below)
* `--role`: IAM role ARN to assume

**Value input methods (in order of precedence):**

1. `--value` flag
1. Piped stdin (e.g., `echo "newvalue" | params2env modify ...`)
1. Interactive prompt

Example:

```bash
params2env modify --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --description "Secret stored as SecureString" \
  --value "S3cr3t" \
  --role "arn:aws:iam::111122223333:role/my-role"

# Pipe value
echo "NewValue" | params2env modify --path "/my/secret"

# Interactive
params2env modify --path "/my/secret"
```

### Subcommand: delete

* `--region`: AWS region (or use `AWS_REGION` env var)
* `--replica`: Replica region to delete from
* `--path`: Parameter path (required)
* `--role`: IAM role ARN to assume

Example:

```bash
params2env delete --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --role "arn:aws:iam::111122223333:role/my-role"
```

### YAML configuration file reference

Config file locations (local overrides global):

* Global: `~/.params2env.yaml`
* Local: `.params2env.yaml`

```yaml
region: <aws region>
replica: <replica region>
prefix: <search path prefix>
file: <output file>
format: <output format: env or github-env>
upper: <uppercase env names, default true>
env_prefix: <env var prefix>
role: <iam role to assume>
kms: <kms key for SecureString>
params:
  - name: <param path>
    env: <custom env var name>
    region: <region override>
    output: <output format override>
```

#### Config fields by command

| Config Field | `create` | `modify` | `delete` | `read` |
|--------------|----------|----------|----------|--------|
| `region` | ✓ | ✓ | ✓ | ✓ |
| `replica` | ✓ | ✓ | ✓ | - |
| `role` | ✓ | ✓ | ✓ | ✓ |
| `kms` | ✓ | - | - | - |
| `format` | - | - | - | ✓ |
| `env_prefix` | - | - | - | ✓ |
| `file` | - | - | - | ✓ |
| `upper` | - | - | - | ✓ |
| `params` | - | - | - | ✓ |

Notes:

* `kms` is only needed for `create` with SecureString.
  Modify/delete don't change encryption.
* `replica` keeps parameters in sync across regions during write operations.
* `read` fetches from one region only.
  Use per-param `region` override for multi-region reads.

#### Example: Simplify commands with config

Without config, creating a SecureString requires many flags:

```bash
params2env create --path /app/secret --value "s3cr3t" --type SecureString \
  --region eu-central-1 --replica eu-west-1 \
  --role arn:aws:iam::123456789012:role/my-role --kms alias/my-key
```

With this config file:

```yaml
region: eu-central-1
replica: eu-west-1
role: arn:aws:iam::123456789012:role/my-role
kms: alias/my-key
```

Commands simplify to:

```bash
params2env create --path /app/secret --value "s3cr3t" --type SecureString
params2env create --path /app/secret --type SecureString  # prompts for value
params2env modify --path /app/secret --value "new-value"
params2env modify --path /app/secret                      # prompts for value
params2env delete --path /app/secret
```

#### Example: Batch read with config

```yaml
region: eu-central-1
role: arn:aws:iam::123456789012:role/my-role
env_prefix: APP
format: github-env
upper: true
params:
  - name: /app/db_password              # → APP_DB_PASSWORD (auto)
  - name: /app/api_token                # → APP_API_TOKEN (auto)
  - name: /legacy/weird-name.v2
    env: LEGACY_KEY                     # → LEGACY_KEY (override)
  - name: /other/endpoint
    region: us-east-1                   # → APP_ENDPOINT (different region)
```

```bash
params2env read                       # Read all from config (github-env format)
params2env read --file ~/.env         # Write to file (github-env format)
params2env read --format env          # Override to traditional shell format
params2env read --path /custom/param  # Single param (ignores params list)
```

## Build and Test

```bash
make build   # Build binary
make tests   # Run tests with coverage
make clean   # Remove binary and coverage files
```

### Integration Tests

`tests/integration-tests.sh` tests against real AWS.

```bash
export AWS_ACCOUNT_ID="123456789012"
export PRIMARY_REGION="eu-central-1"
export SECONDARY_REGION="eu-west-1"
export AWS_IAM_PRINCIPAL="arn:aws:iam::123456789012:role/YourRole"

# Optional: use existing KMS keys instead of creating new ones
export PRIMARY_KEY_ID="xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx"
export REPLICA_KEY_ID="yyyyyyyy-yyyy-yyyy-yyyy-yyyyyyyyyyyy"

./tests/integration-tests.sh
```

Creates IAM roles/policies, tests all param types, cleans up after.
KMS tests cost $1/month per key - script asks before creating.
Set `PRIMARY_KEY_ID` and `REPLICA_KEY_ID` to use existing keys.

## Source

Primary: [git.sr.ht/~wombelix/params2env](https://git.sr.ht/~wombelix/params2env)

Mirrors: [Codeberg](https://codeberg.org/wombelix/params2env),
[Gitlab](https://gitlab.com/wombelix/params2env),
[GitHub](https://github.com/wombelix/params2env)

## Contribute

Please don't hesitate to provide feedback,
open an issue, or create a Pull / Merge Request.

Just pick the workflow or platform you prefer and are most comfortable with.

Feedback, bug reports, or patches sent to my sr.ht list
[~wombelix/inbox@lists.sr.ht](https://lists.sr.ht/~wombelix/inbox) or via
[Email and Instant Messaging](https://dominik.wombacher.cc/pages/contact.html)
are also always welcome.

## License

Unless otherwise stated: `Apache 2.0`

All files contain license information either as a
`header comment` or a `corresponding .license` file.

[REUSE](https://reuse.software) from the [FSFE](https://fsfe.org/)
is implemented to verify license and copyright compliance.
