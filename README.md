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

## Usage

Global flags:

* `--loglevel`: `debug`, `info`, `warn`, `error`, `fatal`, `panic` (default: `info`)
* `--version`: Print version
* `--help`: Print help

### Subcommand: read

* `--region`: AWS region (or use `AWS_REGION` env var)
* `--path`: Parameter path (required)
* `--role`: IAM role ARN to assume
* `--file`: Output file (default: stdout)
* `--upper`: Uppercase env var names (default: `true`)
* `--env-prefix`: Prefix for env var names
* `--env`: Custom env var name (default: parameter name)

Example:

```bash
params2env read --region "eu-central-1" --path "/my/secret" \
  --role "arn:aws:iam::111122223333:role/my-role" \
  --file "~/.my-secret" --upper "false" \
  --env-prefix "my_" --env "secret"
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
* `--value`: Parameter value (required)
* `--type`: `String` or `SecureString` (default: `String`)
* `--kms`: KMS Key ID for SecureString (e.g., `alias/myapp-key`)
* `--role`: IAM role ARN to assume
* `--overwrite`: Overwrite existing (default: `false`)

Example:

```bash
params2env create --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --description "Secret stored as SecureString" \
  --value "S3cr3t" --type "SecureString" \
  --kms "alias/myapp-key" \
  --role "arn:aws:iam::111122223333:role/my-role"
```

### Subcommand: modify

* `--region`: AWS region (or use `AWS_REGION` env var)
* `--replica`: Replica region
* `--path`: Parameter path (required)
* `--description`: Parameter description
* `--value`: New value (required)
* `--role`: IAM role ARN to assume

Example:

```bash
params2env modify --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --description "Secret stored as SecureString" \
  --value "S3cr3t" \
  --role "arn:aws:iam::111122223333:role/my-role"
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

### YAML config file

Per-param settings override globals.

```yaml
region: <aws region>
replica: <replica region>
prefix: <search path prefix>
file: <output file>
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

Example:

```yaml
region: eu-central-1
role: arn:aws:iam::123456789012:role/my-role
env_prefix: APP_
upper: true
params:
  - name: /app/db/url
    env: DB_URL
    region: us-east-1  # Override region for this parameter
  - name: /app/db/user
    env: DB_USER
  - name: /app/db/password
    env: DB_PASSWORD
```

```bash
params2env read                    # Read all from config
params2env read --file ~/.env      # Write to file
params2env read --path /custom/param  # Override config
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

./tests/integration-tests.sh
```

Creates IAM roles/policies, tests all param types, cleans up after.
KMS tests cost $1/month per key - script asks before creating.

## Source

Primary: [git.sr.ht/~wombelix/params2env](https://git.sr.ht/~wombelix/params2env)

Mirrors: [Codeberg](https://codeberg.org/wombelix/params2env),
[Gitlab](https://gitlab.com/wombelix/params2env),
[GitHub](https://github.com/wombelix/params2env)

## Contribute

Issues, PRs, patches welcome. Pick your preferred platform.

Also via [~wombelix/inbox@lists.sr.ht](https://lists.sr.ht/~wombelix/inbox)
or [contact](https://dominik.wombacher.cc/pages/contact.html).

## License

MIT unless stated otherwise. [REUSE](https://reuse.software) compliant.
