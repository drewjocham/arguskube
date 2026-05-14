#!/usr/bin/env bash
# ──────────────────────────────────────────────────────────────────
# GKE Sleep Schedule Helper
# ──────────────────────────────────────────────────────────────────
# Creates Cloud Scheduler jobs to scale the node pool to 0 at night
# and restore it in the morning. Run this after `terraform apply`.
#
# Usage:
#   ./sleep.sh dev     # creates scheduler jobs for dev cluster
#   ./sleep.sh prod    # creates scheduler jobs for prod cluster
#
# Requires: gcloud CLI, project configured
# ──────────────────────────────────────────────────────────────────
set -euo pipefail

ENV="${1:-dev}"
PROJECT_ID="$(gcloud config get project)"
REGION="us-west1"
CLUSTER_NAME="kubewatcher-${ENV}"
NODE_POOL="${CLUSTER_NAME}-pool"

echo "Setting up sleep schedule for ${CLUSTER_NAME} in ${PROJECT_ID}..."

# Service account for scheduler
SA_NAME="scheduler-${ENV}"
SA_EMAIL="${SA_NAME}@${PROJECT_ID}.iam.gserviceaccount.com"

gcloud iam service-accounts describe "${SA_EMAIL}" 2>/dev/null || \
  gcloud iam service-accounts create "${SA_NAME}" \
    --display-name="Sleep scheduler for ${CLUSTER_NAME}"

gcloud projects add-iam-policy-binding "${PROJECT_ID}" \
  --member="serviceAccount:${SA_EMAIL}" \
  --role="roles/container.clusterAdmin" \
  --condition="None" \
  --quiet

# Enable APIs
gcloud services enable cloudscheduler.googleapis.com --quiet

# Sleep: scale node pool min to 0
gcloud scheduler jobs create http "${CLUSTER_NAME}-sleep" \
  --schedule="0 5 * * *" \
  --time-zone="America/Los_Angeles" \
  --uri="https://container.googleapis.com/v1/projects/${PROJECT_ID}/locations/${REGION}/clusters/${CLUSTER_NAME}/nodePools/${NODE_POOL}:setAutoscaling" \
  --http-method="POST" \
  --headers="Content-Type=application/json" \
  --message-body='{"autoscaling":{"minNodeCount":0,"maxNodeCount":3}}' \
  --oidc-service-account-email="${SA_EMAIL}" \
  --oidc-token-audience="https://container.googleapis.com" \
  2>/dev/null || \
  echo "  (sleep job already exists, skipping)"

# Wake: restore node pool min
gcloud scheduler jobs create http "${CLUSTER_NAME}-wake" \
  --schedule="0 13 * * 1-5" \
  --time-zone="America/Los_Angeles" \
  --uri="https://container.googleapis.com/v1/projects/${PROJECT_ID}/locations/${REGION}/clusters/${CLUSTER_NAME}/nodePools/${NODE_POOL}:setAutoscaling" \
  --http-method="POST" \
  --headers="Content-Type=application/json" \
  --message-body='{"autoscaling":{"minNodeCount":1,"maxNodeCount":3}}' \
  --oidc-service-account-email="${SA_EMAIL}" \
  --oidc-token-audience="https://container.googleapis.com" \
  2>/dev/null || \
  echo "  (wake job already exists, skipping)"

echo ""
echo "  ✓ Sleep: scales to 0 at 5pm PT weekdays"
echo "  ✓ Wake: restores at 1pm PT weekdays"
echo "  Cluster: ${CLUSTER_NAME}"
echo "  Zone: ${REGION}"
echo ""
