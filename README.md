<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# AWS SSM Parameter Store to Environment variables

[![REUSE status](https://api.reuse.software/badge/git.sr.ht/~wombelix/params2env)](https://api.reuse.software/info/git.sr.ht/~wombelix/params2env)
[![builds.sr.ht status](https://builds.sr.ht/~wombelix/params2env.svg)](https://builds.sr.ht/~wombelix/params2env?)

## Table of Contents

* [CLI](#cli)
   * [Technical details](#technical-details)
* [Usage](#usage)
   * [Subcommand: read](#subcommand-read)
   * [Subcommand: create](#subcommand-create)
   * [Subcommand: modify](#subcommand-modify)
   * [Subcommand: delete](#subcommand-delete)
   * [YAML configuration file reference](#yaml-configuration-file-reference)
* [Source](#source)
* [Contribute](#contribute)
* [License](#license)

## CLI

`params2env` is a CLI tool to read AWS SSM Parameter Store parameters and write
them to environment variables. It also offers subcommands to create and modify
parameters in the Parameter Store.

### Technical details

It leverages the latest version of the AWS Go SDK to authenticate, assume roles
and interact with parameters from the Parameter Store. A design goal is to prefer
Go standard libraries, for example slog or testing, to avoid external
dependencies, and to achieve a 100% unit test coverage.

The tool can run with a yaml configuration file, ~/.params2env.yaml or
.params2env.yaml, or with command line arguments. Command line arguments have a
higher precedence than the configuration file in the current directory. The
configuration file in the current directory has a higher precedence than the
global configuration file in the home directory.

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

Result:

~/.my-secret

```bash
export MY_SECRET="S3cr3t"
```

When `--output` is set to `env` or removed from the arguments, the output needs
to be evaluated by the shell to set the environment variable. You can do this in
two ways:

```bash
# Using eval
eval $(params2env read --path "/my/secret")

# Using source
source <(params2env read --path "/my/secret")
```

This will set the environment variable in your current shell:

```bash
MY_SECRET="S3cr3t"
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

Settings under params have a higher precedence than the global settings.
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

# Result:
export APP_DB_URL="postgresql://db.example.com:5432"
export APP_DB_USER="dbuser"
export APP_DB_PASSWORD="secret"

# Write all parameters to file
params2env read --file ~/.env

# Override config and read single parameter
params2env read --path /custom/param
```

## Source

The primary location is:
[git.sr.ht/~wombelix/params2env](https://git.sr.ht/~wombelix/params2env)

Mirrors are available on:

* [Codeberg](https://codeberg.org/wombelix/params2env)
* [Gitlab](https://gitlab.com/wombelix/params2env)
* [GitHub](https://github.com/wombelix/params2env)

## Contribute

Please don't hesitate to provide Feedback, open an Issue or create a Pull /
Merge Request.

Just pick the workflow or platform you prefer and are most comfortable with.

Feedback, bug reports or patches to my sr.ht list
[~wombelix/inbox@lists.sr.ht](https://lists.sr.ht/~wombelix/inbox) or via
[Email and Instant Messaging](https://dominik.wombacher.cc/pages/contact.html)
are also always welcome.

## License

Unless otherwise stated: `MIT`

All files contain license information either as `header comment` or
`corresponding .license` file.

[REUSE](https://reuse.software) from the [FSFE](https://fsfe.org/) implemented
to verify license and copyright compliance.
