<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# Usage Instructions

## Installation

```bash
go install git.sr.ht/~wombelix/params2env@latest
```

## Configuration

The tool supports YAML configuration files with this precedence:

1. Command line arguments (highest priority)
1. Local configuration file (./.params2env.yaml)
1. Global configuration file (~/.params2env.yaml)

### Configuration File Format

```yaml
region: eu-central-1
role: arn:aws:iam::123456789012:role/my-role
env_prefix: APP_
upper: true
params:
  - name: /app/db/url
    env: DB_URL
  - name: /app/db/user
    env: DB_USER
  - name: /app/db/password
    env: DB_PASSWORD
```

## Commands

### Read Parameters

```bash
# Basic usage
params2env read --path "/my/secret"

# With custom environment variable name
params2env read --path "/my/secret" --env "MY_SECRET"

# Write to file
params2env read --path "/my/secret" --file "~/.secrets"

# With role assumption
params2env read --path "/my/secret" \
  --role "arn:aws:iam::123456789012:role/my-role"

# Read all parameters from config
params2env read
```

### Create Parameters

```bash
# String parameter
params2env create --path "/my/param" --value "hello"

# SecureString with KMS
params2env create --path "/my/secret" --value "s3cret" \
  --type SecureString --kms "alias/myapp-key"

# With replication
params2env create --path "/my/param" --value "hello" \
  --region "eu-central-1" --replica "eu-west-1"
```

### Modify Parameters

```bash
# Basic modification
params2env modify --path "/my/param" --value "new-value"

# With description
params2env modify --path "/my/param" --value "new-value" \
  --description "Updated parameter"
```

## Environment Variables

The tool respects standard AWS SDK environment variables:

* `AWS_REGION`: Default region if not specified
* `AWS_PROFILE`: AWS profile to use
* Standard AWS credential environment variables

## Common Tasks

### Managing Application Secrets

```bash
# Create secrets
params2env create --path "/app/db/password" --value "secret123" --type SecureString
params2env create --path "/app/api/key" --value "abc123" --type SecureString

# Export to environment
eval $(params2env read --path "/app/db/password" --env "DB_PASSWORD")
eval $(params2env read --path "/app/api/key" --env "API_KEY")
```

### Using Configuration Files

Create `.params2env.yaml`:

```yaml
region: eu-central-1
role: arn:aws:iam::123456789012:role/my-role
env_prefix: APP_
params:
  - name: /app/db/url
    env: DB_URL
  - name: /app/db/password
    env: DB_PASSWORD
```

Then run:

```bash
# Read all configured parameters
params2env read

# Write to file
params2env read --file .env
```

## Troubleshooting

**Region Issues:** Ensure AWS_REGION is set or use --region flag

**Permission Issues:** Check AWS credentials and IAM role permissions

**File Output Issues:** Verify file path permissions and parent directories exist
