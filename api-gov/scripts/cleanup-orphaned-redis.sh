#!/bin/bash
# Cleanup orphaned Redis keys for deleted specs.
# Run as a cron job: */30 * * * * /path/to/cleanup-orphaned-redis.sh
#
# Orphaned keys are Redis keys whose spec_id doesn't exist in PostgreSQL.
# This script finds them using SCAN (not KEYS) to avoid blocking Redis.

set -euo pipefail

REDIS_CLI="${REDIS_CLI:-redis-cli}"
PSQL="${PSQL:-psql}"
DRY_RUN="${DRY_RUN:-true}"
DATABASE_URL="${DATABASE_URL:-postgres://api-gov:api-gov@localhost:5432/api-gov?sslmode=disable}"

# Patterns for keys containing spec_id (second colon-separated field)
PATTERNS=(
  "anomaly:hll:*"
  "anomaly:rate:*"
  "anomaly:status:*"
  "anomaly:fields:*"
  "anomaly:stats:*"
  "anomaly:latency:*"
  "traffic:*"
  "profile:user:*"
  "checkpoint:architect-*"
  "checkpoint:ingest-*"
  "checkpoint:scan-*"
  "checkpoint:hacker-*"
  "checkpoint:heal-*"
  "checkpoint:orchestrator-*"
)

# Extract spec_id from key format: prefix:spec_id:rest or prefix:spec_id
# e.g. anomaly:hll:spec-123:endpoints -> spec-123
extract_spec_id() {
  local key="$1"
  echo "$key" | awk -F: '{print $3}'
}

# Validate spec_id is a UUID
is_uuid() {
  echo "$1" | grep -qE '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
}

echo "=== Redis Orphaned Key Cleanup ==="
echo "DRY_RUN=$DRY_RUN"
echo ""

total_deleted=0
for pattern in "${PATTERNS[@]}"; do
  echo "Scanning pattern: $pattern"
  
  # Use SCAN for non-blocking iteration
  cursor=0
  while true; do
    result=$($REDIS_CLI SCAN "$cursor" MATCH "$pattern" COUNT 1000)
    cursor=$(echo "$result" | head -1)
    keys=$(echo "$result" | tail -n +2)
    
    [ -z "$keys" ] && break
    
    for key in $keys; do
      spec_id=$(extract_spec_id "$key")
      
      # Skip non-UUID keys (checkpoint keys use thread_id format)
      if ! is_uuid "$spec_id"; then
        continue
      fi
      
      # Check if spec exists in PostgreSQL
      exists=$($PSQL "$DATABASE_URL" -t -c "SELECT EXISTS(SELECT 1 FROM api_specs WHERE id='$spec_id')")
      
      if [ "$exists" = " f" ]; then
        if [ "$DRY_RUN" = "true" ]; then
          echo "  [DRY-RUN] Would DELETE $key (spec $spec_id not found)"
        else
          $REDIS_CLI DEL "$key" > /dev/null
          echo "  DELETED $key"
        fi
        total_deleted=$((total_deleted + 1))
      fi
    done
    
    [ "$cursor" = "0" ] && break
  done
done

echo ""
echo "Total keys processed: $total_deleted"
if [ "$DRY_RUN" = "true" ]; then
  echo "Run with DRY_RUN=false to actually delete."
fi
