# Terraform Architecture and Organization

This document provides a visual overview of the Terraform architecture and organization for the Argus project, using Mermaid diagrams.

## 1. High-Level Repository Structure

This graph illustrates how the repository separates reusable modules from live environment configurations.

```mermaid
graph TD
    TF_ROOT[deploy/terraform] --> BOOTSTRAP[bootstrap]
    TF_ROOT --> MODULES[modules]
    TF_ROOT --> LIVE[live]

    subgraph "Bootstrap (State Backend)"
        BOOTSTRAP --> B_AWS[aws]
        BOOTSTRAP --> B_GCP[gcp]
    end

    subgraph "Reusable Modules"
        MODULES --> M_EKS[eks-platform]
        MODULES --> M_GKE[gke-platform]
        MODULES --> M_DOKS[doks-platform]
        MODULES --> M_CR[cloud-run]
        MODULES --> M_FLINK[flink-vm]
        MODULES --> M_HELM[argus-helm]
        MODULES --> M_KW[kubewatcher-helm]
    end

    subgraph "Live Environments"
        LIVE --> L_DEV[dev]
        LIVE --> L_STAGING[staging]
        LIVE --> L_PROD[prod]
        
        L_DEV --> D_GCP[gcp]
        L_DEV --> D_DO[do]
        L_DEV --> D_CR[gcp-cloud-run]
        
        L_PROD --> P_AWS[aws]
        L_PROD --> P_GCP[gcp]
        L_PROD --> P_DO[do]
    end
```

## 2. Module Dependency Map

This diagram demonstrates how the live environments consume custom modules to deploy the infrastructure and applications (Helm charts) across cloud providers.

```mermaid
graph LR
    subgraph "Cloud Platforms"
        EKS(eks-platform)
        GKE(gke-platform)
        DOKS(doks-platform)
        CR(cloud-run)
        FLINK(flink-vm)
    end

    subgraph "Application Layer (Helm)"
        ARGUS(argus-helm)
        KUBEWATCHER(kubewatcher-helm)
    end

    %% Providers mapping
    AWS_ENV{AWS Environments} --> EKS
    GCP_ENV{GCP Environments} --> GKE
    GCP_ENV --> CR
    GCP_ENV --> FLINK
    DO_ENV{DigitalOcean Environments} --> DOKS

    %% Platform to Helm links
    EKS -->|Deploys to| ARGUS
    EKS -->|Deploys to| KUBEWATCHER
    GKE -->|Deploys to| ARGUS
    GKE -->|Deploys to| KUBEWATCHER
    DOKS -->|Deploys to| ARGUS
    DOKS -->|Deploys to| KUBEWATCHER
```

## 3. AWS Production Architecture (Example)

Based on the `live/prod/aws/main.tf` configuration, this represents the actual architecture deployed when applying the AWS Prod stack.

```mermaid
graph TD
    subgraph AWS["AWS Cloud (us-west-2)"]
        subgraph VPC["VPC (10.20.0.0/16)"]
            subgraph Public["Public Subnets (10.20.10x.0/24)"]
                NAT[NAT Gateways]
            end
            
            subgraph Private["Private Subnets (10.20.x.0/24)"]
                subgraph EKS["EKS Cluster (argus-prod)"]
                    NODES["Node Group (m5.large)"]
                    
                    subgraph Helm["argus-helm"]
                        FE[Frontend]
                        BE[Backend]
                        MON[Monitoring]
                        AG[Agent]
                        AI[Alert Ingress]
                        MCP[MCP]
                    end
                end
            end
        end
    end

    Public -->|Internet Access| Private
    NODES --> Helm
```
