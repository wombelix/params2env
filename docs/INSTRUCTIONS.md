<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# params2env - Usage Instructions and Examples

## Overview

`params2env` is a CLI tool for interacting with AWS SSM Parameter Store. It
supports reading parameters and exporting them as environment variables or files,
as well as creating and modifying parameters.

## Installation

```bash
go install git.sr.ht/~wombelix/params2env@latest
```

## Configuration

The tool supports configuration through YAML files with the following precedence:

1. Command line arguments (highest priority)
1. Local configuration file (./.params2env.yaml)
1. Global configuration file (~/.params2env.yaml)

### Configuration File Format

```yaml
region: eu-central-1              # Default AWS region
replica: eu-west-1                # Default replica region for create/modify
prefix: /my/params               # Base path for parameters
output: env                      # Default output format (env or file)
file: ~/.my-secrets             # Default file path for file output
upper: true                      # Convert env var names to uppercase
env_prefix: MY_                  # Prefix for environment variable names
role: arn:aws:iam::123:role/x   # Default role to assume
kms_key: arn:aws:kms:...        # Default KMS key for SecureString
params:                         # Parameter-specific configurations
  - name: /my/secret
    env: SECRET_KEY
    region: us-east-1
    output: file
```

## Commands

### Global Options

* `--loglevel`: Set logging level (debug, info, warn, error), default "info"
* `--version`: Show version information
* `--help`: Show help message

### Read Command

Read a parameter and export it as an environment variable or write it to a file.

```bash
# Basic usage - export as environment variable
params2env read --path "/my/secret"

# Read with custom environment variable name
params2env read --path "/my/secret" --env "MY_SECRET"

# Write to file with custom path
params2env read \
  --path "/my/secret" \
  --output file \
  --file "~/.secrets/my-secret"

# Read with role assumption and custom region
params2env read \
  --path "/my/secret" \
  --region "eu-central-1" \
  --role "arn:aws:iam::123456789012:role/my-role"

# Read with prefix and uppercase name
params2env read \
  --path "/my/secret" \
  --env-prefix "APP_" \
  --upper true
```

### Create Command

Create a new parameter in the Parameter Store.

```bash
# Create a String parameter
params2env create \
  --path "/my/param" \
  --value "hello" \
  --type String

# Create a SecureString with KMS encryption
params2env create \
  --path "/my/secret" \
  --value "s3cret" \
  --type SecureString \
  --kms-id "arn:aws:kms:region:account:key/id" \
  --description "My secret parameter"

# Create with replica in another region
params2env create \
  --path "/my/param" \
  --value "hello" \
  --type String \
  --region "eu-central-1" \
  --replica "eu-west-1"

# Create with role assumption
params2env create \
  --path "/my/param" \
  --value "hello" \
  --type String \
  --role "arn:aws:iam::123456789012:role/my-role"
```

### Modify Command

Modify an existing parameter in the Parameter Store.

```bash
# Basic modification
params2env modify \
  --path "/my/param" \
  --value "new-value"

# Modify with description
params2env modify \
  --path "/my/param" \
  --value "new-value" \
  --description "Updated parameter"

# Modify with replica
params2env modify \
  --path "/my/param" \
  --value "new-value" \
  --region "eu-central-1" \
  --replica "eu-west-1"
```

## Environment Variables

The tool respects the following environment variables:

* `AWS_REGION`: Default AWS region if not specified in config or flags
* Standard AWS SDK environment variables for authentication

## Best Practices

1. **Security:**
   * Use SecureString type for sensitive values
   * Always specify a KMS key for SecureString parameters
   * Use role assumption for cross-account access

1. **Configuration:**
   * Use configuration files for common settings
   * Override specific values with command line flags
   * Keep sensitive parameters in separate paths

1. **Naming:**
   * Use consistent parameter paths
   * Use descriptive environment variable names
   * Consider using prefixes for related parameters

## Troubleshooting

1. **Region Issues:**
   * Ensure AWS_REGION is set if not using --region flag
   * Verify region in configuration files

1. **Permission Issues:**
   * Check AWS credentials are properly configured
   * Verify IAM roles have necessary permissions
   * For SecureString, ensure KMS key access

1. **File Output Issues:**
   * Check file path permissions
   * Ensure parent directories exist
   * Verify file permissions (defaults to 0600)

## Examples of Common Tasks

### Managing Application Secrets

```bash
# Create secrets
params2env create --path "/app/prod/db/password" --value "secret123" --type SecureString
params2env create --path "/app/prod/api/key" --value "abc123" --type SecureString

# Export to environment
params2env read --path "/app/prod/db/password" --env "DB_PASSWORD"
params2env read --path "/app/prod/api/key" --env "API_KEY"

# Write to file for docker-compose
params2env read --path "/app/prod/db/password" --output file --file ".env"
```

### Cross-Region Replication

```bash
# Create parameter with replica
params2env create \
  --path "/app/global/config" \
  --value "shared-value" \
  --type String \
  --region "eu-central-1" \
  --replica "eu-west-1"

# Update in both regions
params2env modify \
  --path "/app/global/config" \
  --value "new-value" \
  --region "eu-central-1" \
  --replica "eu-west-1"
```

### Using with Docker

```bash
# Export multiple parameters
params2env read --path "/app/prod/db/url" --env "DB_URL"
params2env read --path "/app/prod/db/user" --env "DB_USER"
params2env read --path "/app/prod/db/password" --env "DB_PASSWORD"

# Run container with exported variables
docker run --env-file <(params2env read --path "/app/prod/db/password") myapp
```
