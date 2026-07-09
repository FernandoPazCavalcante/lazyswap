# Infrastructure & IaC

## Cloud Provider

**Azure** — Only cloud infrastructure is defined for azure-infra repo; no AWS, GCP, or hybrid deployments.

## Infrastructure as Code

**Terraform** — HCL-based IaC for azure-infra; two independent modules:

### `init/` Module (Bootstrap)
- Creates Azure Storage Account used as remote state backend for `infra/`
- Runs once; state stored in Terraform Cloud (workspace `olorinlabs-infra-init`, org `OlorinLabs`)
- Remote state: `terraform.tfstate` in TFC

### `infra/` Module (Application)
- Per-environment deployment via `-var-file` (dev.tfvars, prod.tfvars)
- Remote state: Azure Storage Account (keys: `dev.terraform.tfstate`, `prod.terraform.tfstate`)

## Provisioned Resources (infra/ Module)

| Resource | Count | Details |
|---|---|---|
| **Resource Group** | 1 per env | Scoped per dev/prod |
| **Key Vault** | 1 per env | Auto-generated MySQL password, JWT secret; User Assigned Managed Identity access |
| **MySQL Flexible Server** | 1 per env | Public endpoints (no VNet); firewall rule `[redacted-ip]/[redacted-ip]` allows Azure services; SSL required |
| **Container App Environment** | 1 per env | Consumption plan (no VNet injection) |
| **Container App** | 1 per env | .NET API on port 8080; scale-to-zero in dev, min 2 replicas in prod |
| **Static Web App** | Optional | Vue.js frontend; Free tier; gated by `deploy_frontend` boolean |
| **Container Registry** | Optional | Toggled by `use_azure_container_registry`; for dev/test |
| **Log Analytics Workspace** | 1 per env | Diagnostic logs for Container App and MySQL |

## Environment Strategy

| Aspect | dev | prod |
|---|---|---|
| **MySQL SKU** | `B_Standard_B1ms` | `GP_Standard_D2ds_v4` |
| **MySQL HA** | Disabled | Enabled |
| **Container image source** | Docker Hub | Azure Container Registry (ACR) |
| **Min replicas** | 1 | 2 (enforced in locals.tf) |
| **Key Vault network** | `Allow` | `Deny` |
| **Resource locks** | None | RG + MySQL locked |
| **Log retention** | 30 days | 90 days |

## Naming Convention

`{resource-abbr}-{project}-{env}-{location_short}` with 6-char random suffix on globally-unique resources (Key Vault, MySQL, ACR).

## Authentication

**Azure Service Principal** (`sp-olorinlabs-terraform`) with roles:
- Contributor
- Owner (required for azurerm_management_lock)

Credentials via Terraform Cloud environment variables:
- `ARM_CLIENT_ID`
- `ARM_CLIENT_SECRET` (sensitive)
- `ARM_SUBSCRIPTION_ID`
- `ARM_TENANT_ID`

## Constraints

- Backend key must match environment (`dev.terraform.tfstate` / `prod.terraform.tfstate`)
- Destroying prod requires manual removal of two resource locks
- Single IP per firewall rule (not CIDR ranges); each value maps to both `start_ip_address` and `end_ip_address`

## Container Orchestration

**Azure Container Apps** (serverless) — No Kubernetes clusters; Consumption plan for free-tier cost optimisation.

## Containerisation

**Docker** — phoenix has multi-stage Dockerfile for Python app; other repos not containerised.

## Helm

**flagsmith-charts** repo provides Helm charts for Flagsmith feature flag platform deployment (optional external integration).

