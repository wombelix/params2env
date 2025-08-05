<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# AWS SSM Parameter Store to Environment variables

[![REUSE status](https://api.reuse.software/badge/git.sr.ht/~wombelix/params2env)](https://api.reuse.software/info/git.sr.ht/~wombelix/params2env)
[![builds.sr.ht status](https://builds.sr.ht/~wombelix/params2env.svg)](https://builds.sr.ht/~wombelix/params2env?)

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

`params2env` is a CLI tool to read AWS SSM Parameter Store parameters and write
them to environment variables. It also offers subcommands to create and modify
parameters in the Parameter Store.

### Technical details

The tool uses the AWS Go SDK to authenticate, assume roles
and interact with parameters from the Parameter Store. I tried to keep external
dependencies minimal and stick to Go standard libraries like slog and testing
where possible. The goal is 100% unit test coverage.

You can run it with a YAML configuration file (~/.params2env.yaml or
.params2env.yaml) or with command line arguments. Command line arguments
override config file settings. A config file in the current directory
takes precedence over the global one in your home directory.

## Installation

Pre-built binaries for Linux, macOS, and Windows are available on
the [GitHub Releases](https://github.com/wombelix/params2env/releases) page.

Download the binary for your platform and add it to your PATH.

## Usage

Global arguments:

* `--loglevel <optional>`: The log level, either `debug`, `info`, `warn`,
  `error`, `fatal`, `panic`, default is `info`
* `--version <optional>`: Print version and exit
* `--help <optional>`: Print help and exit

### Subcommand: read

Arguments:

* `--region <optional>`: The AWS region to use, optional because it can also be
  set via env var `AWS_REGION`
* `--path <required>`: The full path to the parameter in the Parameter Store
* `--role <optional>`: The role to assume to read the parameter
* `--file <optional>`: The file to write to (if not specified, prints to stdout)
* `--upper <optional>`: The environment variable names are upper case, either
  `true` or `false`, default is `true`
* `--env-prefix <optional>`: The prefix to append to the environment variable
  names, default is empty
* `--env <optional>`: The environment variable to set, if not defined then
  parameter name

Example:

```bash
params2env read --region "eu-central-1" --path "/my/secret" \
  --role "arn:aws:iam::111122223333:role/my-role" \
  --file "~/.my-secret" --upper "false" \
  --env-prefix "my_" --env "secret"
```

Result (Example values, no actual secrets):

~/.my-secret

```bash
export MY_SECRET="<secret-value>"
```

To actually set the environment variables in your shell, you need to evaluate
the output. You can do this in two ways:

```bash
# Using eval
eval $(params2env read --path "/my/secret")

# Using source
source <(params2env read --path "/my/secret")
```

This will set the environment variable in your current
shell (Example values, no actual secrets):

```bash
MY_SECRET="<secret-value>"
```

### Subcommand: create

Arguments:

* `--region <optional>`: The AWS region to use, optional because it can also be
  set via env var `AWS_REGION`
* `--replica <optional>`: The AWS region to use for the replica entry
* `--path <required>`: The full path to the parameter in the Parameter Store
* `--description <optional>`: The description of the parameter
* `--value <required>`: The value of the parameter
* `--type <optional>`: The type of the parameter, either `String` or
  `SecureString`, default is `String`
* `--kms <optional>`: The KMS Key ID for SecureString parameters, use
  alias/myapp-key format for customer managed keys
* `--role <optional>`: The role to assume to create the parameter
* `--overwrite <optional>`: Overwrite an existing parameter, either `true` or
  `false`, default is `false`

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

Arguments:

* `--region <optional>`: The AWS region to use, optional because it can also be
  set via env var `AWS_REGION`
* `--replica <optional>`: The AWS region to use for the replica entry
* `--path <required>`: The full path to the parameter in the Parameter Store
* `--description <optional>`: The description of the parameter
* `--value <required>`: The value of the parameter
* `--role <optional>`: The role to assume to modify the parameter

Example:

```bash
params2env modify --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --description "Secret stored as SecureString" \
  --value "S3cr3t" \
  --role "arn:aws:iam::111122223333:role/my-role"
```

### Subcommand: delete

Arguments:

* `--region <optional>`: The AWS region to use, optional because it can also be
  set via env var `AWS_REGION`
* `--replica <optional>`: The AWS region to delete the replica from
* `--path <required>`: The full path to the parameter in the Parameter Store
* `--role <optional>`: The role to assume to delete the parameter

Example:

```bash
params2env delete --region "eu-central-1" --replica "eu-west-1" \
  --path "/my/secret" \
  --role "arn:aws:iam::111122223333:role/my-role"
```

### YAML configuration file reference

Settings under params override the global settings.
Some settings are only used when reading parameters, others when writing parameters.

```yaml
region: <optional: aws region to use>
replica: <optional: aws region to use for the replica entry>
prefix: <optional: search params by name below this path>
file: <optional: file to write to>
upper: <optional: env var names are upper case, either "true" or "false",
  default is "true">
env_prefix: <optional: prefix to append to env var names>
role: <optional: role to assume to read the parameters>
kms: <optional: KMS Key ID for SecureString parameters>
params:
  - name: <required: full path to the parameter>
    env: <optional: custom environment variable name>
    region: <optional: region-specific override>
    output: <optional: output format override>
  - name: <another parameter>
    env: <another env var name>
    # ... more parameters as needed
```

Example configuration with multiple parameters:

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

Using this configuration:

```bash
# Read all parameters from config
params2env read

# Result (Example values, no actual secrets):
export APP_DB_URL="postgresql://db.example.com:5432"
export APP_DB_USER="dbuser"
export APP_DB_PASSWORD="<password-value>"

# Write all parameters to file
params2env read --file ~/.env

# Override config and read single parameter
params2env read --path /custom/param
```

## Build and Test

### Makefile

The project includes a Makefile with the following targets:

* `make build`: Build the params2env binary
* `make tests`: Run all unit tests with verbose output and coverage report
* `make clean`: Remove the binary and coverage files

```bash
# Build the binary
make build

# Run tests
make tests

# Clean up
make clean
```

### Integration Tests

The `tests/integration-tests.sh` script tests all features against a real AWS environment,
including parameter creation, modification, deletion, and role assumption.

Prerequisites:

* AWS credentials configured
* The following environment variables set:
   * `AWS_ACCOUNT_ID`: Your 12-digit AWS account ID
   * `PRIMARY_REGION`: Your primary AWS region (e.g., eu-central-1)
   * `SECONDARY_REGION`: Your secondary AWS region (e.g., eu-west-1)
   * `AWS_IAM_PRINCIPAL`: Your IAM principal ARN (user or role)

```bash
# Set required environment variables
export AWS_ACCOUNT_ID="123456789012"
export PRIMARY_REGION="eu-central-1"
export SECONDARY_REGION="eu-west-1"
export AWS_IAM_PRINCIPAL="arn:aws:iam::123456789012:role/YourRole"

# Run integration tests
./tests/integration-tests.sh
```

The script will create necessary IAM roles and policies, test String and SecureString
parameters with and without role assumption, test configuration file functionality,
and clean up all created resources.

Note: Some tests involve AWS KMS keys which incur costs ($1/month per key).
The script will ask for confirmation before creating any billable resources.

## Source

The primary location is:
[git.sr.ht/~wombelix/params2env](https://git.sr.ht/~wombelix/params2env)

Mirrors are available on
[Codeberg](https://codeberg.org/wombelix/params2env),
[Gitlab](https://gitlab.com/wombelix/params2env)
and
[GitHub](https://github.com/wombelix/params2env).

## Contribute

Please don't hesitate to provide feedback, open an issue or create a pull /
merge request.

Just pick the workflow or platform you prefer and are most comfortable with.

Feedback, bug reports or patches to my sr.ht list
[~wombelix/inbox@lists.sr.ht](https://lists.sr.ht/~wombelix/inbox) or via
[email and instant messaging](https://dominik.wombacher.cc/pages/contact.html)
are also always welcome.

## License

Unless otherwise stated: `MIT`

All files contain license information either as
`header comment` or `corresponding .license` file.

[REUSE](https://reuse.software) from the [FSFE](https://fsfe.org/)
implemented to verify license and copyright compliance.
