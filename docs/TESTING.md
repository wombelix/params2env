<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# Manual Testing Guide for params2env

## Cost Considerations

Testing this tool will create AWS resources that may incur costs:

- AWS Systems Manager Parameter Store parameters (free tier available)
- AWS KMS keys (if using customer managed keys, ~$1/month per key)
- IAM roles and policies (free)

Estimated total cost for running all tests (assuming cleanup within 1 hour):

- Minimum charge for KMS keys: $2.00 (1 key + 1 replica for the partial month)
- API requests: Negligible (less than $0.01)
- Total: Approximately $2.00

## Environment Setup

### Required Environment Variables

```bash
# Set required environment variables
export AWS_ACCOUNT_ID="123456789012"      # Your AWS account ID
export PRIMARY_REGION="eu-central-1"       # Your primary region
export SECONDARY_REGION="eu-west-1"        # Your secondary region for replication
export AWS_IAM_PRINCIPAL="arn:aws:iam::${AWS_ACCOUNT_ID}:user/your-user"  \
    # Your IAM principal ARN
```

### AWS IAM Setup

Create an IAM role and policy for testing:

```bash
# Create IAM policy
aws iam create-policy \
    --policy-name params2env-test-policy \
    --policy-document '{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "ssm:PutParameter",
                    "ssm:GetParameter",
                    "ssm:GetParameters",
                    "ssm:DeleteParameter",
                    "ssm:DeleteParameters"
                ],
                "Resource": [
                    "arn:aws:ssm:*:'"${AWS_ACCOUNT_ID}"':parameter/test/*"
                ]
            },
            {
                "Effect": "Allow",
                "Action": [
                    "kms:Decrypt",
                    "kms:Encrypt",
                    "kms:GenerateDataKey"
                ],
                "Resource": [
                    "arn:aws:kms:*:'"${AWS_ACCOUNT_ID}"':key/*"
                ]
            }
        ]
    }'

# Create IAM role
ROLE_POLICY='{
    "Version": "2012-10-17",
    "Statement": [{
        "Effect": "Allow",
        "Principal": {"AWS": "'"${AWS_IAM_PRINCIPAL}"'"},
        "Action": "sts:AssumeRole"
    }]
}'

aws iam create-role \
    --role-name params2env-test-role \
    --assume-role-policy-document "${ROLE_POLICY}"

# Attach policy to role
POLICY_NAME="params2env-test-policy"
POLICY_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:policy/${POLICY_NAME}"
aws iam attach-role-policy \
    --role-name params2env-test-role \
    --policy-arn "${POLICY_ARN}"
```

### Optional: Customer Managed KMS Keys Setup

If you want to test with customer managed KMS keys
(instead of AWS managed keys), you can create them
as follows:

```bash
# Create KMS key in primary region
PRIMARY_KEY_ID=$(aws kms create-key --region "${PRIMARY_REGION}" \
    --description "Test key for params2env in ${PRIMARY_REGION}" \
    --query 'KeyMetadata.KeyId' --output text)

PRIMARY_KEY_ARN=$(aws kms describe-key --key-id "${PRIMARY_KEY_ID}" \
    --region "${PRIMARY_REGION}" --query 'KeyMetadata.Arn' --output text)

# Create alias for primary key
aws kms create-alias --alias-name alias/params2env-test \
    --target-key-id "${PRIMARY_KEY_ID}" --region "${PRIMARY_REGION}"

# Create KMS key in secondary region
REPLICA_KEY_ID=$(aws kms create-key --region "${SECONDARY_REGION}" \
    --description "Test key for params2env in ${SECONDARY_REGION}" \
    --query 'KeyMetadata.KeyId' --output text)

# Create alias for replica key
aws kms create-alias --alias-name alias/params2env-test \
    --target-key-id "${REPLICA_KEY_ID}" --region "${SECONDARY_REGION}"

# Export key IDs for later use (if using customer managed keys)
export PRIMARY_KEY_ID REPLICA_KEY_ID PRIMARY_KEY_ARN
```

## Test Commands

### String Parameters

```bash
# Create String parameter (with role)
ROLE_PATH="arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"
params2env create --path "/test/string-param" \
    --value "test-value-1" --type "String" \
    --region "${PRIMARY_REGION}" \
    --replica "${SECONDARY_REGION}" \
    --role "${ROLE_PATH}"

# Create String parameter (without role)
params2env create --path "/test/string-param-no-role" --value "test-value-1" \
    --type "String" --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}"

# Read String parameter (with role)
params2env read --path "/test/string-param" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read String parameter (without role)
params2env read --path "/test/string-param-no-role" --region "${PRIMARY_REGION}"

# Read String parameter (custom env name, with role)
params2env read --path "/test/string-param" --env "TEST_STRING_PARAM" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read String parameter (custom env name, without role)
params2env read --path "/test/string-param-no-role" \
    --env "TEST_STRING_PARAM_NO_ROLE" --region "${PRIMARY_REGION}"

# Read String parameter (with prefix)
params2env read --path "/test/string-param" --env-prefix "TEST" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read a string parameter to a file
params2env read --path "/test/string-param" --file "./test-string.env" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Expected output:
# Reading parameter '/test/string-param' from region 'eu-central-1'
# Parameter value written to ./test-string.env

# Verify file contents
cat ./test-string.env
export STRING_PARAM="test-value-1"

# Read multiple parameters to file
params2env read --path "/test/string-param" --path "/test/string-param-no-role" \
    --file "./test-multiple.env" --region "${PRIMARY_REGION}"

# Expected output:
# Reading parameter /test/string-param from region eu-central-1
# Reading parameter /test/string-param-no-role from region eu-central-1
# Parameter values written to ./test-multiple.env

# Verify multiple parameters file contents
cat ./test-multiple.env

# Expected output:
# export STRING_PARAM="test-value-1"
# export STRING_PARAM_NO_ROLE="test-value-1"

# Modify String parameter
params2env modify --path "/test/string-param" --value "test-value-2" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Delete String parameter (with role)
params2env delete --path "/test/string-param" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Delete String parameter (without role)
params2env delete --path "/test/string-param-no-role" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}"
```

### SecureString Parameters with AWS Managed KMS Key

```bash
# Create SecureString parameter with AWS managed key
params2env create --path "/test/secure-param-aws" --value "secure-value-1" \
    --type "SecureString" --kms "alias/aws/ssm" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (default output)
params2env read --path "/test/secure-param-aws" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (custom env name)
params2env read --path "/test/secure-param-aws" \
    --env "TEST_SECURE_PARAM_AWS" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (with prefix)
params2env read --path "/test/secure-param-aws" --env-prefix "TEST" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (to file)
params2env read --path "/test/secure-param-aws" \
    --file "./test-secure-aws.env" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Modify SecureString parameter
params2env modify --path "/test/secure-param-aws" --value "secure-value-2" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Delete SecureString parameter
params2env delete --path "/test/secure-param-aws" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"
```

### SecureString Parameters with Customer Managed KMS Key

```bash
# Create SecureString parameter with custom key (primary region)
KMS_KEY="arn:aws:kms:${PRIMARY_REGION}:${AWS_ACCOUNT_ID}:key"
KMS_ARN="${KMS_KEY}/${PRIMARY_KEY_ID}"
params2env create \
    --path "/test/secure-param-custom" \
    --value "secure-value-1" \
    --type "SecureString" \
    --kms "${KMS_ARN}" \
    --region "${PRIMARY_REGION}" \
    --replica "${SECONDARY_REGION}"

# Expected output:
# Successfully created parameter '/test/secure-param-custom' in region 'eu-central-1'
# Successfully created parameter '/test/secure-param-custom' in replica region 'eu-west-1'

# Read SecureString parameter (default output)
params2env read --path "/test/secure-param-custom" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (custom env name)
params2env read --path "/test/secure-param-custom" \
    --env "TEST_SECURE_PARAM_CUSTOM" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (with prefix)
params2env read --path "/test/secure-param-custom" --env-prefix "TEST" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Read SecureString parameter (to file)
params2env read --path "/test/secure-param-custom" \
    --file "./test-secure-custom.env" --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Expected output:
# Reading parameter /test/secure-param-custom from region eu-central-1
# Parameter value written to ./test-secure-custom.env

# Verify file contents
cat ./test-secure-custom.env

# Expected output:
# export SECURE_PARAM_CUSTOM="secure-value-1"

# Modify SecureString parameter
params2env modify --path "/test/secure-param-custom" \
    --value "secure-value-2" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Delete SecureString parameters
params2env delete --path "/test/secure-param-custom" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"
```

### Configuration File Tests

Create a test configuration file `.params2env.yaml.template`:

```yaml
region: ${PRIMARY_REGION}
replica: ${SECONDARY_REGION}
role: arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role
env_prefix: TEST
upper: true
params:
  - name: /test/param1
    env: PARAM_ONE
    region: ${PRIMARY_REGION}
  - name: /test/param2
    env: PARAM_TWO
    region: ${SECONDARY_REGION}
```

Note: Before using the configuration file, replace the environment
variable placeholders with their actual values:

```bash
# Create configuration file from template
envsubst < .params2env.yaml.template > .params2env.yaml
```

Test commands with configuration:

```bash
# Create parameters defined in config
params2env create --path "/test/param1" --value "config-value-1" --type "String"
params2env create --path "/test/param2" --value "config-value-2" --type "String"

# Read all parameters from config
params2env read

# Read all parameters to file
params2env read --file "./test-config.env"

# Modify parameters
params2env modify --path "/test/param1" --value "config-value-1-modified"
params2env modify --path "/test/param2" --value "config-value-2-modified"

# Delete parameters
params2env delete --path "/test/param1"
params2env delete --path "/test/param2"
```

## Cleanup

Remove test resources:

```bash
# Delete test parameters if any remain
params2env delete --path "/test/string-param" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

params2env delete --path "/test/secure-param-aws" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

params2env delete --path "/test/secure-param-custom" \
    --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

params2env delete --path "/test/param1" \
    --region "${PRIMARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

params2env delete --path "/test/param2" \
    --region "${SECONDARY_REGION}" \
    --role "arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Delete test files
rm -f ./test-string.env ./test-secure-aws.env ./test-secure-custom.env \
    ./test-config.env .params2env.yaml

# Remove KMS aliases and keys (if you created them)
aws kms delete-alias --alias-name alias/params2env-test \
    --region "${PRIMARY_REGION}"
aws kms delete-alias --alias-name alias/params2env-test \
    --region "${SECONDARY_REGION}"
aws kms schedule-key-deletion --key-id "${PRIMARY_KEY_ID}" \
    --pending-window-in-days 7 --region "${PRIMARY_REGION}"
aws kms schedule-key-deletion --key-id "${REPLICA_KEY_ID}" \
    --pending-window-in-days 7 --region "${SECONDARY_REGION}"

# Remove IAM role and policy
aws iam detach-role-policy \
    --role-name params2env-test-role \
    --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"

aws iam delete-role \
    --role-name params2env-test-role

aws iam delete-policy \
    --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"

# Unset environment variables
unset AWS_ACCOUNT_ID PRIMARY_REGION SECONDARY_REGION AWS_IAM_PRINCIPAL \
    PRIMARY_KEY_ID PRIMARY_KEY_ARN REPLICA_KEY_ID
```

## Expected Results

1. String parameter tests should:

   - Create parameter in both regions
   - Show parameter value in various output formats
   - Successfully modify the value
   - Delete the parameter from both regions

1. SecureString parameter tests should:

   - Create encrypted parameter in both regions
   - Show decrypted parameter value in various output formats
   - Successfully modify the value
   - Delete the parameter from both regions
   - Work with both AWS managed KMS key and customer managed key

1. Configuration file tests should:

   - Create all parameters defined in the config
   - Read all parameters in a single command
   - Successfully modify all parameters
   - Delete all parameters

1. Cleanup commands should execute without errors, including:

   - Parameter deletion
   - File cleanup
   - KMS key cleanup (with 7-day deletion window)
   - IAM role and policy cleanup
