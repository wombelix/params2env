<!--
SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>

SPDX-License-Identifier: CC0-1.0
-->

# Manual Testing Guide

## Cost Warning

Testing creates AWS resources that may incur costs:

- KMS keys (~$1/month per key if using customer managed keys)
- Parameter Store operations (mostly free tier)

Estimated cost for full test run: ~$2 if cleaned up within an hour.

## Setup

Set required environment variables:

```bash
export AWS_ACCOUNT_ID="123456789012"
export PRIMARY_REGION="eu-central-1"
export SECONDARY_REGION="eu-west-1"
export AWS_IAM_PRINCIPAL="arn:aws:iam::${AWS_ACCOUNT_ID}:user/your-user"
```

Create test IAM role:

```bash
# Create policy
aws iam create-policy --policy-name params2env-test-policy \
  --policy-document '{
    "Version": "2012-10-17",
    "Statement": [{
      "Effect": "Allow",
      "Action": [
      "ssm:*Parameter*", "kms:Decrypt", "kms:Encrypt", "kms:GenerateDataKey"
    ],
      "Resource": [
      "arn:aws:ssm:*:'${AWS_ACCOUNT_ID}':parameter/test/*",
      "arn:aws:kms:*:'${AWS_ACCOUNT_ID}':key/*"
    ]
    }]
  }'

# Create role
aws iam create-role --role-name params2env-test-role \
--assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [{
    "Effect": "Allow",
    "Principal": {"AWS": "'${AWS_IAM_PRINCIPAL}'"},
    "Action": "sts:AssumeRole"
  }]
}'

# Attach policy
aws iam attach-role-policy --role-name params2env-test-role \
  --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"
```

## Test Commands

### String Parameters

```bash
ROLE_ARN="arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role"

# Create and test String parameter
params2env create --path "/test/string-param" --value "test-value" \
  --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" --role "${ROLE_ARN}"

params2env read --path "/test/string-param" --region "${PRIMARY_REGION}" --role "${ROLE_ARN}"

# Test file output
params2env read --path "/test/string-param" --file "./test.env" \
  --region "${PRIMARY_REGION}" --role "${ROLE_ARN}"

cat ./test.env  # Should show: export STRING_PARAM="test-value"

# Modify and delete
params2env modify --path "/test/string-param" --value "new-value" \
  --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" --role "${ROLE_ARN}"

params2env delete --path "/test/string-param" \
  --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" --role "${ROLE_ARN}"
```

### SecureString Parameters

```bash
# With AWS managed KMS key
params2env create --path "/test/secure-param" --value "secret123" \
  --type SecureString --kms "alias/aws/ssm" \
  --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" --role "${ROLE_ARN}"

params2env read --path "/test/secure-param" --region "${PRIMARY_REGION}" --role "${ROLE_ARN}"

params2env delete --path "/test/secure-param" \
  --region "${PRIMARY_REGION}" --replica "${SECONDARY_REGION}" --role "${ROLE_ARN}"
```

### Configuration File Test

Create `.params2env.yaml`:

```bash
cat > .params2env.yaml << EOF
region: ${PRIMARY_REGION}
role: arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role
env_prefix: TEST_
params:
  - name: /test/config1
    env: PARAM_ONE
  - name: /test/config2
    env: PARAM_TWO
EOF
```

Test with config:

```bash
# Create parameters
params2env create --path "/test/config1" --value "config-value-1"
params2env create --path "/test/config2" --value "config-value-2"

# Read all from config
params2env read

# Read to file
params2env read --file "./config-test.env"

# Clean up
params2env delete --path "/test/config1"
params2env delete --path "/test/config2"
```

## Cleanup

```bash
# Remove test files
rm -f ./test.env ./config-test.env .params2env.yaml

# Remove IAM resources
aws iam detach-role-policy --role-name params2env-test-role \
  --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"
aws iam delete-role --role-name params2env-test-role
aws iam delete-policy \
  --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"

# Unset variables
unset AWS_ACCOUNT_ID PRIMARY_REGION SECONDARY_REGION AWS_IAM_PRINCIPAL
```

## Expected Results

All commands should execute without errors. Parameters should be created
in both regions when using `--replica`, and file outputs should contain
properly formatted environment variable exports.
