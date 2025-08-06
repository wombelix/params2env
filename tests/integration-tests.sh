#!/usr/bin/env bash

# SPDX-FileCopyrightText: 2025 Dominik Wombacher <dominik@wombacher.cc>
#
# SPDX-License-Identifier: MIT

# Exit on error, but ensure we run cleanup first
set -e

# Color codes for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color
WHITE='\033[1;37m'

# Global variables
CLEANUP_NEEDED=false
IAM_RESOURCES_CREATED=false
KMS_KEYS_CREATED=false
USE_CUSTOM_KMS=false

validate_environment() {
    local missing_vars=()

    # Check required environment variables
    if [ -z "${AWS_ACCOUNT_ID}" ]; then
        missing_vars+=("AWS_ACCOUNT_ID")
    elif ! [[ "${AWS_ACCOUNT_ID}" =~ ^[0-9]{12}$ ]]; then
        echo -e "${RED}Error: AWS_ACCOUNT_ID must be a 12-digit number${NC}"
        return 1
    fi

    if [ -z "${PRIMARY_REGION}" ]; then
        missing_vars+=("PRIMARY_REGION")
    elif ! [[ "${PRIMARY_REGION}" =~ ^[a-z]{2}-[a-z]+-[0-9]$ ]]; then
        echo -e "${RED}Error: PRIMARY_REGION must be a valid AWS region (e.g., eu-central-1)${NC}"
        return 1
    fi

    if [ -z "${SECONDARY_REGION}" ]; then
        missing_vars+=("SECONDARY_REGION")
    elif ! [[ "${SECONDARY_REGION}" =~ ^[a-z]{2}-[a-z]+-[0-9]$ ]]; then
        echo -e "${RED}Error: SECONDARY_REGION must be a valid AWS region (e.g., eu-west-1)${NC}"
        return 1
    fi

    # Auto-detect AWS_IAM_PRINCIPAL if not set
    if [ -z "${AWS_IAM_PRINCIPAL}" ]; then
        echo -e "${YELLOW}AWS_IAM_PRINCIPAL not set, attempting to detect from current identity...${NC}"

        local current_identity
        if ! current_identity=$(aws sts get-caller-identity --query 'Arn' --output text 2>/dev/null); then
            echo -e "${RED}Error: Failed to get current AWS identity. Please ensure AWS credentials are configured.${NC}"
            return 1
        fi

        echo -e "${YELLOW}Current identity: $current_identity${NC}"

        # Convert assumed role to base role ARN
        if [[ $current_identity =~ ^arn:aws:sts::[0-9]{12}:assumed-role/([^/]+)/ ]]; then
            local role_name="${BASH_REMATCH[1]}"
            local account_id
            account_id=$(echo "$current_identity" | cut -d: -f5)
            AWS_IAM_PRINCIPAL="arn:aws:iam::${account_id}:role/${role_name}"
            echo -e "${GREEN}Auto-detected IAM principal: ${AWS_IAM_PRINCIPAL}${NC}"
        elif [[ $current_identity =~ ^arn:aws:iam::[0-9]{12}:(user|role)/ ]]; then
            AWS_IAM_PRINCIPAL="$current_identity"
            echo -e "${GREEN}Auto-detected IAM principal: ${AWS_IAM_PRINCIPAL}${NC}"
        else
            echo -e "${RED}Error: Unable to determine IAM principal from current identity${NC}"
            echo -e "${YELLOW}Current identity: $current_identity${NC}"
            echo -e "${YELLOW}Please set AWS_IAM_PRINCIPAL manually to a valid IAM user or role ARN${NC}"
            return 1
        fi

        export AWS_IAM_PRINCIPAL
    else
        # Validate provided AWS_IAM_PRINCIPAL
        if [[ $AWS_IAM_PRINCIPAL =~ ^arn:aws:sts::[0-9]{12}:assumed-role/([^/]+)/ ]]; then
            local role_name="${BASH_REMATCH[1]}"
            local account_id
            account_id=$(echo "$AWS_IAM_PRINCIPAL" | cut -d: -f5)
            AWS_IAM_PRINCIPAL="arn:aws:iam::${account_id}:role/${role_name}"
            echo -e "${YELLOW}Converting assumed role to base role ARN: ${AWS_IAM_PRINCIPAL}${NC}"
        elif ! [[ $AWS_IAM_PRINCIPAL =~ ^arn:aws:iam::[0-9]{12}:(user|role)/.+ ]]; then
            echo -e "${RED}Error: AWS_IAM_PRINCIPAL must be a valid IAM ARN${NC}"
            return 1
        fi
    fi

    # If any other variables are missing, print them and return error
    if [ ${#missing_vars[@]} -ne 0 ]; then
        echo -e "${RED}Error: Missing required environment variables:${NC}"
        for var in "${missing_vars[@]}"; do
            echo -e "${YELLOW}  - $var${NC}"
        done
        echo -e "\nPlease set them before running this script:"
        echo -e "${GREEN}export AWS_ACCOUNT_ID=\"123456789012\"  # Your AWS account ID"
        echo -e "export PRIMARY_REGION=\"eu-central-1\"   # Your primary region"
        echo -e "export SECONDARY_REGION=\"eu-west-1\"    # Your secondary region${NC}"
        echo -e "${YELLOW}# AWS_IAM_PRINCIPAL will be auto-detected from your current AWS identity${NC}"
        return 1
    fi

    return 0
}

# Function to clean up SSM parameters in a region
cleanup_ssm_parameters() {
    local region=$1
    echo -e "${YELLOW}Cleaning up parameters in ${region}...${NC}"

    # Get all parameters under /params2env-test/ path recursively as JSON
    local params_json
    params_json=$(aws ssm get-parameters-by-path \
        --path "/params2env-test/" \
        --recursive \
        --region "${region}" \
        --query 'Parameters[].Name' \
        --output json 2>/dev/null)

    if [ -n "$params_json" ] && [ "$params_json" != "[]" ]; then
        # Parse JSON array and process each parameter
        while IFS= read -r param; do
            if [ -n "$param" ]; then
                echo -n "  - Deleting: $param ... "
                if aws ssm delete-parameter --name "$param" --region "${region}" 2>/dev/null; then
                    echo -e "${GREEN}✓${NC}"
                else
                    echo -e "${RED}✗${NC}"
                fi
            fi
        done < <(echo "$params_json" | jq -r '.[]')

        # Verify all parameters are deleted
        local remaining_json
        remaining_json=$(aws ssm get-parameters-by-path \
            --path "/params2env-test/" \
            --recursive \
            --region "${region}" \
            --query 'Parameters[].Name' \
            --output json 2>/dev/null)

        if [ -n "$remaining_json" ] && [ "$remaining_json" != "[]" ]; then
            echo -e "${RED}Warning: Some parameters could not be deleted in ${region}:${NC}"
            echo "$remaining_json" | jq -r '.[]'
        else
            echo -e "${GREEN}All parameters successfully deleted in ${region}${NC}"
        fi
    else
        echo -e "${GREEN}No parameters found under /params2env-test/ in ${region}${NC}"
    fi
}

# Cleanup function to be called on script exit
cleanup() {
    if [ "$CLEANUP_NEEDED" = true ]; then
        echo -e "\n${YELLOW}Performing cleanup due to script interruption...${NC}"

        # Clean up SSM parameters in both regions
        cleanup_ssm_parameters "${PRIMARY_REGION}"
        cleanup_ssm_parameters "${SECONDARY_REGION}"

        if [ "$IAM_RESOURCES_CREATED" = true ]; then
            cleanup_iam || true
        fi

        if [ "$KMS_KEYS_CREATED" = true ]; then
            cleanup_kms || true
        fi

        # Clean up any test files that might have been created
        rm -f ./test-string.env ./test-secure-aws.env ./test-secure-custom.env ./test-config.env .params2env.yaml .params2env.yaml.template || true

        echo -e "${GREEN}Cleanup completed${NC}"
    fi
}

# Set up trap for cleanup on script exit
trap cleanup EXIT

# Function to check if a variable is set and validate its value
check_var() {
    local var_name=$1
    local var_value=${!var_name}
    local is_valid=true

    while true; do
        if [ -z "$var_value" ]; then
            echo -e "${YELLOW}$var_name is not set.${NC}"
            read -r -p "Please enter value for $var_name: " var_value
            is_valid=false
        fi

        case $var_name in
            AWS_ACCOUNT_ID)
                if ! [[ $var_value =~ ^[0-9]{12}$ ]]; then
                    echo -e "${RED}Invalid AWS account ID. Must be 12 digits.${NC}"
                    read -r -p "Please enter a valid AWS account ID: " var_value
                    is_valid=false
                    continue
                fi
                ;;
            PRIMARY_REGION|SECONDARY_REGION)
                if ! aws ec2 describe-regions --query "Regions[?RegionName=='$var_value'].RegionName" --output text > /dev/null 2>&1; then
                    echo -e "${RED}Invalid AWS region: $var_value${NC}"
                    read -r -p "Please enter a valid AWS region: " var_value
                    is_valid=false
                    continue
                fi
                ;;
            AWS_IAM_PRINCIPAL)
                # Get current identity
                local current_identity
                current_identity=$(aws sts get-caller-identity --query 'Arn' --output text)

                if [ -z "$var_value" ] || [ "$is_valid" = false ]; then
                    echo -e "${YELLOW}Current AWS identity: $current_identity${NC}"
                    echo -e "${YELLOW}For IAM users, use the user ARN directly.${NC}"
                    echo -e "${YELLOW}For assumed roles (e.g., AWS SSO), use the role ARN (not the assumed-role ARN).${NC}"
                    echo -e "${YELLOW}Example: arn:aws:iam::123456789012:user/john.doe${NC}"
                    read -r -p "Please enter the correct IAM principal ARN: " var_value
                    is_valid=false
                    continue
                fi

                # Validate ARN format
                if ! [[ $var_value =~ ^arn:aws:iam::[0-9]{12}:(user|role)/.+ ]]; then
                    echo -e "${RED}Invalid IAM principal ARN format.${NC}"
                    echo -e "${YELLOW}Current identity: $current_identity${NC}"
                    read -r -p "Please enter a valid IAM principal ARN: " var_value
                    is_valid=false
                    continue
                fi
                ;;
        esac

        # If we get here, the value is valid
        break
    done

    # Export the validated value
    export "$var_name"="$var_value"
    echo -e "${GREEN}$var_name has been set to: $var_value${NC}"

    # Ask for confirmation
    read -r -p "Is this value correct? [Y/n] " confirm
    if [[ $confirm =~ ^[Nn]$ ]]; then
        check_var "$var_name" ""
    fi
}

# Function to prompt for yes/no with default
prompt_yes_no() {
    local prompt="$1"
    local default="$2"
    local REPLY

    # Print prompt in white (not yellow) and keep cursor on same line
    echo -en "${WHITE}${prompt}${NC} "
    read -r REPLY

    # Convert empty response to default
    if [ -z "$REPLY" ]; then
        REPLY=$default
    fi

    # Convert to lowercase
    echo "$REPLY" | tr '[:upper:]' '[:lower:]'
}

# Function to check for existing resources
check_existing_resources() {
    local found_resources=()

    # Check for parameters in primary region
    local primary_json
    primary_json=$(aws ssm get-parameters-by-path \
        --path "/params2env-test/" \
        --recursive \
        --region "${PRIMARY_REGION}" \
        --query 'Parameters[].Name' \
        --output json 2>/dev/null)

    if [ -n "$primary_json" ] && [ "$primary_json" != "[]" ]; then
        found_resources+=("SSM Parameters in ${PRIMARY_REGION} under /params2env-test/")
        while IFS= read -r param; do
            if [ -n "$param" ]; then
                found_resources+=("  - ${param}")
            fi
        done < <(echo "$primary_json" | jq -r '.[]')
    fi

    # Check for parameters in secondary region
    local secondary_json
    secondary_json=$(aws ssm get-parameters-by-path \
        --path "/params2env-test/" \
        --recursive \
        --region "${SECONDARY_REGION}" \
        --query 'Parameters[].Name' \
        --output json 2>/dev/null)

    if [ -n "$secondary_json" ] && [ "$secondary_json" != "[]" ]; then
        found_resources+=("SSM Parameters in ${SECONDARY_REGION} under /params2env-test/")
        while IFS= read -r param; do
            if [ -n "$param" ]; then
                found_resources+=("  - ${param}")
            fi
        done < <(echo "$secondary_json" | jq -r '.[]')
    fi

    # If resources were found, ask to clean them up
    if [ ${#found_resources[@]} -gt 0 ]; then
        echo -e "\n${YELLOW}Found existing test resources:${NC}"
        printf '%s\n' "${found_resources[@]}"
        echo
        echo -en "${WHITE}Do you want to clean up these resources before proceeding? [Y/n] ${NC}"
        read -r cleanup_response
        cleanup_response=${cleanup_response:-y}
        if [[ "$cleanup_response" =~ ^[Yy]$ ]]; then
            cleanup_existing_resources

            # Verify cleanup was successful
            local remaining_primary
            remaining_primary=$(aws ssm get-parameters-by-path --path "/params2env-test/" --recursive --region "${PRIMARY_REGION}" --query 'Parameters[].Name' --output json 2>/dev/null)
            local remaining_secondary
            remaining_secondary=$(aws ssm get-parameters-by-path --path "/params2env-test/" --recursive --region "${SECONDARY_REGION}" --query 'Parameters[].Name' --output json 2>/dev/null)

            if { [ -n "$remaining_primary" ] && [ "$remaining_primary" != "[]" ]; } || { [ -n "$remaining_secondary" ] && [ "$remaining_secondary" != "[]" ]; }; then
                echo -e "${RED}Error: Some resources could not be cleaned up. Please check and clean up manually before proceeding.${NC}"
                exit 1
            fi
        fi
    fi
}

# Function to clean up existing resources
cleanup_existing_resources() {
    echo -e "\n${GREEN}Cleaning up existing resources...${NC}"

    # Clean up SSM parameters in both regions
    cleanup_ssm_parameters "${PRIMARY_REGION}"
    cleanup_ssm_parameters "${SECONDARY_REGION}"

    # Clean up IAM policy if it exists
    if aws iam get-policy --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy" >/dev/null 2>&1; then
        echo "Cleaning up IAM policy..."
        cleanup_iam
    fi
}

# Function to create IAM policy
create_iam_policy() {
    echo -e "\n${GREEN}=== Creating IAM Policy ===${NC}"

    # Check if policy already exists
    if aws iam get-policy --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy" >/dev/null 2>&1; then
        echo -e "${YELLOW}Policy params2env-test-policy already exists, reusing existing policy${NC}"
        return 0
    fi

    # Create policy document
    local policy_document='{
        "Version": "2012-10-17",
        "Statement": [
            {
                "Effect": "Allow",
                "Action": [
                    "ssm:PutParameter",
                    "ssm:GetParameter",
                    "ssm:DeleteParameter",
                    "ssm:AddTagsToResource",
                    "ssm:RemoveTagsFromResource",
                    "ssm:ListTagsForResource"
                ],
                "Resource": [
                    "arn:aws:ssm:*:'${AWS_ACCOUNT_ID}':parameter/params2env-test/*"
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
                    "arn:aws:kms:*:'${AWS_ACCOUNT_ID}':key/*"
                ]
            }
        ]
    }'

    # Create policy
    if ! aws iam create-policy \
        --policy-name params2env-test-policy \
        --policy-document "$policy_document" --no-cli-pager; then
        echo -e "${RED}Failed to create IAM policy${NC}"
        exit 1
    fi

    echo -e "${GREEN}Successfully created IAM policy${NC}"
}

# Function to print resource information
# shellcheck disable=SC2120
print_resource_info() {
    if [ "$#" -gt 0 ]; then
        echo "Warning: print_resource_info doesn't take any arguments" >&2
        return 1
    fi

    echo "This script will create and manage ONLY the following AWS resources:"
    echo
    echo "1. IAM Resources:"
    echo "   - Role: params2env-test-role"
    echo "   - Policy: params2env-test-policy"
    echo "   Policy will be scoped to:"
    echo "   - SSM parameters under /params2env-test/* only"

    if [ -n "${PRIMARY_KEY_ID}" ] && [ -n "${REPLICA_KEY_ID}" ]; then
        echo "   - Using existing KMS keys:"
        echo "     Primary (${PRIMARY_REGION}): ${PRIMARY_KEY_ID}"
        echo "     Replica (${SECONDARY_REGION}): ${REPLICA_KEY_ID}"
    else
        echo "   - Using AWS managed KMS keys"
    fi

    echo
    echo "2. SSM Parameters:"
    echo "   All parameters will be created under /params2env-test/* prefix:"
    echo "   - /params2env-test/string-param"
    echo "   - /params2env-test/string-param-no-role"
    echo "   - /params2env-test/secure-param-aws"
    if [ -n "${PRIMARY_KEY_ID}" ] && [ -n "${REPLICA_KEY_ID}" ]; then
        echo "   - /params2env-test/secure-param-custom"
    fi
    echo "   - /params2env-test/param1"
    echo "   - /params2env-test/param2"

    if [ -n "${PRIMARY_KEY_ID}" ] && [ -n "${REPLICA_KEY_ID}" ]; then
        echo
        echo "Note: Using existing KMS keys, no new keys will be created"
    else
        echo
        echo "Cost Warning:"
        echo "- Each KMS key costs $1.00/month"
        echo "- Total cost for two keys: $2.00/month"
        echo "- Keys can be scheduled for deletion after testing"
    fi

    echo
    echo -en "${WHITE}Do you want to proceed? [y/N] ${NC}"
    read -r proceed
    if [[ ! $proceed =~ ^[Yy]$ ]]; then
        echo "Aborting."
        exit 0
    fi
}

# Function to check for existing KMS keys
check_existing_kms_keys() {
    # Try to get existing key IDs from environment
    if [ -n "${PRIMARY_KEY_ID}" ] && [ -n "${REPLICA_KEY_ID}" ]; then
        echo "Existing KMS keys found:"
        echo "PRIMARY_KEY_ID: ${PRIMARY_KEY_ID}"
        echo "REPLICA_KEY_ID: ${REPLICA_KEY_ID}"
        echo -en "${WHITE}Do you want to use these existing keys? [Y/n] ${NC}"
        read -r use_existing
        use_existing=${use_existing:-y}
        if [[ "$use_existing" =~ ^[Yy]$ ]]; then
            return 0
        fi
    fi

    # Ask about creating new customer managed keys
    echo -en "${WHITE}Do you want to test with customer managed KMS keys? [y/N] ${NC}"
    read -r create_custom
    if [[ "$create_custom" =~ ^[Yy]$ ]]; then
        echo
        echo "Cost Warning:"
        echo "- Each KMS key costs $1.00/month"
        echo "- Total cost for two keys: $2.00/month"
        echo "- Keys can be scheduled for deletion after testing"
        echo
        echo -en "${WHITE}Do you want to proceed with creating KMS keys? [y/N] ${NC}"
        read -r proceed
        if [[ "$proceed" =~ ^[Yy]$ ]]; then
            setup_kms_keys
            return 0
        fi
    fi
    return 1
}

# Function to run a command and check its exit status
run_cmd() {
    local cmd=$1
    local description=$2

    echo -n "Running: $description"
    echo "$cmd"

    if eval "$cmd"; then
        echo -e "${GREEN}✓ Success${NC}"
        return 0
    else
        echo -e "${RED}✗ Failed${NC}"
        return 1
    fi
}

# Function to find the params2env binary
find_binary() {
    local binary_path

    # Check current directory first
    if [ -x "./params2env" ]; then
        binary_path="./params2env"
    else
        # Try to find in PATH
        binary_path=$(command -v params2env 2>/dev/null)
    fi

    if [ -z "$binary_path" ]; then
        echo -e "${RED}Error: params2env binary not found in current directory or PATH${NC}"
        echo "Please ensure the binary is built and available in the current directory or PATH"
        return 1
    fi

    if [ ! -x "$binary_path" ]; then
        echo -e "${RED}Error: $binary_path is not executable${NC}"
        return 1
    fi

    echo "$binary_path"
}

# Function to create IAM role
create_iam_role() {
    echo -e "\n${GREEN}=== Creating IAM Role ===${NC}"

    # Check if role already exists
    if aws iam get-role --role-name params2env-test-role >/dev/null 2>&1; then
        echo -e "${YELLOW}Role params2env-test-role already exists, reusing existing role${NC}"
        return 0
    fi

    # Create trust policy document with proper JSON escaping
    local trust_policy
    # Convert SSO role ARN to proper format if needed
    local formatted_principal="$AWS_IAM_PRINCIPAL"
    if [[ "$AWS_IAM_PRINCIPAL" =~ ^arn:aws:iam::[0-9]{12}:role/AWSReservedSSO_ ]]; then
        formatted_principal="arn:aws:iam::${AWS_ACCOUNT_ID}:root"
    fi

    trust_policy=$(jq -n \
        --arg principal "$formatted_principal" \
        '{
            Version: "2012-10-17",
            Statement: [{
                Effect: "Allow",
                Principal: {
                    AWS: $principal
                },
                Action: "sts:AssumeRole"
            }]
        }')

    # Create role
    if ! aws iam create-role \
        --role-name params2env-test-role \
        --assume-role-policy-document "$trust_policy" --no-cli-pager; then
        echo -e "${RED}Failed to create IAM role${NC}"
        exit 1
    fi

    # Attach policy to role
    if ! aws iam attach-role-policy \
        --role-name params2env-test-role \
        --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy" --no-cli-pager; then
        echo -e "${RED}Failed to attach policy to role${NC}"
        exit 1
    fi

    echo -e "${GREEN}Successfully created IAM role${NC}"
}

# Function to clean up IAM resources
cleanup_iam() {
    echo -e "${YELLOW}Cleaning up IAM resources...${NC}"

    # Detach policy from role
    aws iam detach-role-policy \
        --role-name params2env-test-role \
        --policy-arn "arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy" 2>/dev/null || true

    # Delete role
    aws iam delete-role \
        --role-name params2env-test-role 2>/dev/null || true

    # Delete policy versions
    policy_arn="arn:aws:iam::${AWS_ACCOUNT_ID}:policy/params2env-test-policy"
    versions=$(aws iam list-policy-versions --policy-arn "$policy_arn" --query 'Versions[?!IsDefaultVersion].VersionId' --output text 2>/dev/null || echo "")
    for version in $versions; do
        aws iam delete-policy-version --policy-arn "$policy_arn" --version-id "$version" 2>/dev/null || true
    done

    # Delete policy
    aws iam delete-policy \
        --policy-arn "$policy_arn" 2>/dev/null || true
}

# Function to clean up KMS resources in a region
cleanup_kms() {
    local region=$1
    echo -e "${YELLOW}Cleaning up KMS resources in $region...${NC}"

    # Get key ID from alias
    local key_id
    key_id=$(aws kms list-aliases --region "$region" --query "Aliases[?AliasName=='alias/params2env-test'].TargetKeyId" --output text)

    if [ -n "$key_id" ]; then
        # Delete alias
        aws kms delete-alias --alias-name alias/params2env-test --region "$region" || true

        # Schedule key deletion
        aws kms schedule-key-deletion --key-id "$key_id" --pending-window-in-days 7 --region "$region" || true
    fi
}

# Function to configure KMS settings
# shellcheck disable=SC2120
configure_kms() {
    if [ "$#" -gt 0 ]; then
        echo "Warning: configure_kms doesn't take any arguments" >&2
        return 1
    fi

    echo -e "\n${GREEN}=== KMS Configuration ===${NC}"
    echo -e "${YELLOW}Do you want to test with customer managed KMS keys? [y/N]${NC}"
    read -r use_custom_kms_response

    if [[ $use_custom_kms_response =~ ^[Yy]$ ]]; then
        USE_CUSTOM_KMS=true
        # Check if KMS key IDs are already set
        if [ -n "${PRIMARY_KEY_ID:-}" ] && [ -n "${REPLICA_KEY_ID:-}" ]; then
            echo -e "\n${YELLOW}Existing KMS keys found:${NC}"
            echo "PRIMARY_KEY_ID: $PRIMARY_KEY_ID"
            echo "REPLICA_KEY_ID: $REPLICA_KEY_ID"

            # Validate existing keys
            if ! aws kms describe-key --key-id "$PRIMARY_KEY_ID" --region "$PRIMARY_REGION" >/dev/null 2>&1; then
                echo -e "${RED}Primary KMS key not found or not accessible${NC}"
                unset PRIMARY_KEY_ID
            fi
            if ! aws kms describe-key --key-id "$REPLICA_KEY_ID" --region "$SECONDARY_REGION" >/dev/null 2>&1; then
                echo -e "${RED}Replica KMS key not found or not accessible${NC}"
                unset REPLICA_KEY_ID
            fi

            if [ -n "${PRIMARY_KEY_ID:-}" ] && [ -n "${REPLICA_KEY_ID:-}" ]; then
                echo -e "${YELLOW}Do you want to use these existing keys? [Y/n]${NC}"
                read -r use_existing_keys
                if [[ $use_existing_keys =~ ^[Nn]$ ]]; then
                    unset PRIMARY_KEY_ID REPLICA_KEY_ID
                fi
            fi
        fi

        if [ -z "${PRIMARY_KEY_ID:-}" ] || [ -z "${REPLICA_KEY_ID:-}" ]; then
            echo -e "\n${RED}WARNING: Creating new KMS keys will incur additional costs:${NC}"
            echo "- Each KMS key costs approximately $1/month"
            echo "- Two keys will be created (one in each region)"
            echo "- Total cost: ~$2/month"
            echo "- Keys can be scheduled for deletion after testing"
            echo -e "${YELLOW}Do you want to proceed with creating new KMS keys? [y/N]${NC}"
            read -r confirm_create_keys
            if [[ $confirm_create_keys =~ ^[Yy]$ ]]; then
                create_kms_keys
            else
                echo -e "${YELLOW}Skipping KMS key creation, proceeding with AWS managed keys${NC}"
                USE_CUSTOM_KMS=false
            fi
        fi
    fi
}

# Function to get KMS configuration
get_kms_config() {
    local config_file=$1
    cat "$config_file"
    rm -f "$config_file"
}

# Function to validate IAM principal
validate_iam_principal() {
    local principal=$1
    local current_identity
    current_identity=$(aws sts get-caller-identity --query 'Arn' --output text)

    echo -e "\n${GREEN}=== IAM Principal Validation ===${NC}"
    echo -e "${YELLOW}Current AWS identity: $current_identity${NC}"

    if [[ $current_identity =~ assumed-role/.*/.* ]]; then
        # Extract account ID and role name from assumed role
        local account_id role_name
        account_id=$(echo "$current_identity" | cut -d: -f5)
        role_name=$(echo "$current_identity" | sed -n 's/.*assumed-role\/\([^/]*\)\/.*/\1/p')

        echo -e "${YELLOW}You are using an assumed role. For the integration tests, you should use:${NC}"
        echo -e "1. Your IAM user ARN (if you have one)"
        echo -e "2. The base role ARN (not the assumed-role ARN)"
        echo -e "\nExample role ARN format: arn:aws:iam::${account_id}:role/${role_name}"

        read -r -p "Please enter the correct IAM principal ARN: " principal
    fi

    # Validate ARN format
    if ! [[ $principal =~ ^arn:aws:iam::[0-9]{12}:(user|role)/.+ ]]; then
        echo -e "${RED}Invalid IAM principal ARN format.${NC}"
        return 1
    fi

    # Validate principal exists
    if ! aws iam get-role --role-name "${principal#arn:aws:iam::*:role/}" >/dev/null 2>&1 && \
       ! aws iam get-user --user-name "${principal#arn:aws:iam::*:user/}" >/dev/null 2>&1; then
        echo -e "${RED}IAM principal does not exist or is not accessible${NC}"
        return 1
    fi

    echo -e "${GREEN}IAM principal validated successfully${NC}"
    return 0
}

# Main script execution
echo -e "\n${GREEN}=== params2env Integration Tests ===${NC}\n"

# Find params2env binary first
PARAMS2ENV=$(find_binary) || exit 1
echo -e "${GREEN}Using params2env binary: $PARAMS2ENV${NC}"

# Validate environment variables
validate_environment

# Check for existing resources
check_existing_resources

# Configure KMS settings
# shellcheck disable=SC2119
configure_kms

# Print detailed resource information and get confirmation
# shellcheck disable=SC2119
print_resource_info

# Set cleanup flag
CLEANUP_NEEDED=true

# Create IAM resources
create_iam_policy
create_iam_role

# Mark IAM resources as created
IAM_RESOURCES_CREATED=true

# Wait for role to be ready
echo -e "${YELLOW}Waiting for IAM role to be ready...${NC}"
sleep 10

# Create test configuration
echo -e "\n${GREEN}=== Creating Test Configuration ===${NC}"

# Create configuration with proper resource tagging
cat > .params2env.yaml.template << EOF
region: ${PRIMARY_REGION}
replica: ${SECONDARY_REGION}
role: arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role
env_prefix: TEST
upper: true
tags:
  Purpose: params2env-test
params:
  - name: /params2env-test/param1
    env: PARAM_ONE
    region: ${PRIMARY_REGION}
    tags:
      Purpose: params2env-test
  - name: /params2env-test/param2
    env: PARAM_TWO
    region: ${SECONDARY_REGION}
    tags:
      Purpose: params2env-test
EOF

envsubst < .params2env.yaml.template > .params2env.yaml

# Test String Parameters
echo -e "\n${GREEN}=== Testing String Parameters ===${NC}"

# Test with role
run_cmd "$PARAMS2ENV create --path '/params2env-test/string-param' --value 'test-value-1' --type 'String' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}' --role 'arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role'" \
    "Create String parameter (with role)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/string-param' --region '${PRIMARY_REGION}' --role 'arn:aws:iam::${AWS_ACCOUNT_ID}:role/params2env-test-role'" \
    "Read String parameter (with role)"

# Test without role
run_cmd "$PARAMS2ENV create --path '/params2env-test/string-param-no-role' --value 'test-value-1' --type 'String' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
    "Create String parameter (without role)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/string-param-no-role' --region '${PRIMARY_REGION}'" \
    "Read String parameter (without role)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/string-param' --env 'TEST_STRING_PARAM' --region '${PRIMARY_REGION}'" \
    "Read String parameter (custom env name)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/string-param' --env-prefix 'TEST' --region '${PRIMARY_REGION}'" \
    "Read String parameter (with prefix)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/string-param' --file './test-string.env' --region '${PRIMARY_REGION}'" \
    "Read String parameter (to file)"

run_cmd "$PARAMS2ENV modify --path '/params2env-test/string-param' --value 'test-value-2' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
    "Modify String parameter"

# Test SecureString Parameters with AWS Managed Key
echo -e "\n${GREEN}=== Testing SecureString Parameters (AWS Managed Key) ===${NC}"

run_cmd "$PARAMS2ENV create --path '/params2env-test/secure-param-aws' --value 'secure-value-1' --type 'SecureString' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
    "Create SecureString parameter (AWS managed key)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/secure-param-aws' --region '${PRIMARY_REGION}'" \
    "Read SecureString parameter (default output)"

run_cmd "$PARAMS2ENV read --path '/params2env-test/secure-param-aws' --file './test-secure-aws.env' --region '${PRIMARY_REGION}'" \
    "Read SecureString parameter (to file)"

run_cmd "$PARAMS2ENV modify --path '/params2env-test/secure-param-aws' --value 'secure-value-2' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
    "Modify SecureString parameter"

# Test SecureString Parameters with Customer Managed Key (if enabled)
if [[ $USE_CUSTOM_KMS =~ ^[Yy]$ ]]; then
    echo -e "\n${GREEN}=== Testing SecureString Parameters (Customer Managed Key) ===${NC}"

    run_cmd "$PARAMS2ENV create --path '/params2env-test/secure-param-custom' --value 'secure-value-1' --type 'SecureString' --kms 'arn:aws:kms:${PRIMARY_REGION}:${AWS_ACCOUNT_ID}:key/${PRIMARY_KEY_ID}' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
        "Create SecureString parameter (custom key)"

    run_cmd "$PARAMS2ENV read --path '/params2env-test/secure-param-custom' --region '${PRIMARY_REGION}'" \
        "Read SecureString parameter (default output)"

    run_cmd "$PARAMS2ENV read --path '/params2env-test/secure-param-custom' --file './test-secure-custom.env' --region '${PRIMARY_REGION}'" \
        "Read SecureString parameter (to file)"

    run_cmd "$PARAMS2ENV modify --path '/params2env-test/secure-param-custom' --value 'secure-value-2' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
        "Modify SecureString parameter"
fi

# Test Configuration File
echo -e "\n${GREEN}=== Testing Configuration File ===${NC}"

run_cmd "$PARAMS2ENV create --path '/params2env-test/param1' --value 'config-value-1' --type 'String'" \
    "Create first parameter from config"

run_cmd "$PARAMS2ENV create --path '/params2env-test/param2' --value 'config-value-2' --type 'String'" \
    "Create second parameter from config"

run_cmd "$PARAMS2ENV read" \
    "Read all parameters from config"

run_cmd "$PARAMS2ENV read --file './test-config.env'" \
    "Read all parameters to file"

run_cmd "$PARAMS2ENV modify --path '/params2env-test/param1' --value 'config-value-1-modified'" \
    "Modify first parameter"

run_cmd "$PARAMS2ENV modify --path '/params2env-test/param2' --value 'config-value-2-modified'" \
    "Modify second parameter"

# Function to test expected failures
run_cmd_expect_fail() {
    local cmd=$1
    local description=$2

    echo -n "Running: $description"
    echo "$cmd"

    if eval "$cmd" >/dev/null 2>&1; then
        echo -e "${RED}✗ Failed - should have failed but succeeded${NC}"
        return 1
    else
        echo -e "${GREEN}✓ Success - correctly failed as expected${NC}"
        return 0
    fi
}

# Test Error Scenarios
echo -e "\n${GREEN}=== Testing Error Scenarios ===${NC}"

run_cmd_expect_fail "$PARAMS2ENV read --path '/params2env-test/nonexistent-param' --region '${PRIMARY_REGION}'" \
    "Read non-existent parameter (should fail)"

run_cmd_expect_fail "$PARAMS2ENV modify --path '/params2env-test/nonexistent-param' --value 'test' --region '${PRIMARY_REGION}'" \
    "Modify non-existent parameter (should fail)"

run_cmd_expect_fail "$PARAMS2ENV delete --path '/params2env-test/nonexistent-param' --region '${PRIMARY_REGION}'" \
    "Delete non-existent parameter (should fail)"

run_cmd_expect_fail "$PARAMS2ENV read --path '/params2env-test/string-param' --region '${PRIMARY_REGION}' --role 'arn:aws:iam::${AWS_ACCOUNT_ID}:role/nonexistent-role'" \
    "Read with invalid IAM role (should fail)"

# Test parameter value size limits (AWS SSM limit is 4KB for String, 8KB for SecureString)
echo -n "Testing parameter value size limit (should fail): "
echo "Creating parameter with >4KB value"
large_value=$(printf 'A%.0s' {1..5000})  # 5KB value, exceeds 4KB limit
if $PARAMS2ENV create --path '/params2env-test/large-param' --value "$large_value" --type 'String' --region "${PRIMARY_REGION}" >/dev/null 2>&1; then
    echo -e "${RED}✗ Failed - should have failed but succeeded${NC}"
    # Clean up if it somehow succeeded
    $PARAMS2ENV delete --path '/params2env-test/large-param' --region "${PRIMARY_REGION}" >/dev/null 2>&1 || true
else
    echo -e "${GREEN}✓ Success - correctly failed as expected${NC}"
fi

# Test invalid parameter path formats
run_cmd_expect_fail "$PARAMS2ENV create --path 'invalid-path-no-slash' --value 'test' --type 'String' --region '${PRIMARY_REGION}'" \
    "Create parameter with invalid path format (should fail)"

run_cmd_expect_fail "$PARAMS2ENV create --path '/params2env-test/param with spaces' --value 'test' --type 'String' --region '${PRIMARY_REGION}'" \
    "Create parameter with spaces in path (should fail)"

run_cmd_expect_fail "$PARAMS2ENV create --path '/params2env-test/invalid-kms-test' --value 'test' --type 'SecureString' --kms 'arn:aws:kms:${PRIMARY_REGION}:${AWS_ACCOUNT_ID}:key/invalid-key-id' --region '${PRIMARY_REGION}'" \
    "Create SecureString with invalid KMS key (should fail)"

run_cmd_expect_fail "$PARAMS2ENV create --path '/params2env-test/invalid-region-test' --value 'test' --type 'String' --region 'invalid-region-123'" \
    "Create parameter with invalid region (should fail)"

# Test special characters in parameter values
special_chars_value='value with special chars: !@#$%^&*(){}[]|\:;"<>?,./'
run_cmd "$PARAMS2ENV create --path '/params2env-test/special-chars' --value '$special_chars_value' --type 'String' --region '${PRIMARY_REGION}'" \
    "Create parameter with special characters"

run_cmd "$PARAMS2ENV read --path '/params2env-test/special-chars' --region '${PRIMARY_REGION}'" \
    "Read parameter with special characters"

# Cleanup
echo -e "\n${GREEN}=== Cleanup ===${NC}"

# Delete parameters
echo -e "${YELLOW}Cleaning up SSM parameters...${NC}"
for param in "/params2env-test/string-param" "/params2env-test/string-param-no-role" "/params2env-test/secure-param-aws" "/params2env-test/param1" "/params2env-test/param2" "/params2env-test/special-chars"; do
    run_cmd "$PARAMS2ENV delete --path '$param' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
        "Delete parameter: $param" || true
done

if [[ $USE_CUSTOM_KMS =~ ^[Yy]$ ]]; then
    run_cmd "$PARAMS2ENV delete --path '/params2env-test/secure-param-custom' --region '${PRIMARY_REGION}' --replica '${SECONDARY_REGION}'" \
        "Delete SecureString parameter (custom key)" || true
fi

# Delete test files
echo -e "\n${YELLOW}Cleaning up test files...${NC}"
rm -f ./test-string.env ./test-secure-aws.env ./test-secure-custom.env ./test-config.env .params2env.yaml .params2env.yaml.template

# Schedule KMS key deletion if custom keys were created
if [[ $USE_CUSTOM_KMS =~ ^[Yy]$ ]] && [ "$KMS_KEYS_CREATED" = true ]; then
    echo -e "\n${YELLOW}Do you want to schedule KMS keys for deletion? [y/N]${NC}"
    echo -e "${RED}Warning: Keys will be scheduled for deletion in 7 days${NC}"
    read -r delete_kms
    if [[ $delete_kms =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Scheduling KMS keys for deletion...${NC}"
        cleanup_kms "$PRIMARY_REGION"
        cleanup_kms "$SECONDARY_REGION"
    else
        echo -e "${YELLOW}KMS keys will be retained. You can delete them manually later using:${NC}"
        echo "aws kms schedule-key-deletion --key-id $PRIMARY_KEY_ID --pending-window-in-days 7 --region $PRIMARY_REGION"
        echo "aws kms schedule-key-deletion --key-id $REPLICA_KEY_ID --pending-window-in-days 7 --region $SECONDARY_REGION"
    fi
fi

# Cleanup IAM resources
cleanup_iam

echo -e "\n${GREEN}Integration tests completed!${NC}"
