# K8s Deployment

## Prerequisites
- K8s cluster with 8+ CPU nodes
- External Secrets Operator installed
- pgBackRest-compatible S3 bucket for backups

## Deploy
```bash
kubectl apply -k .
kubectl rollout status -n api-gov deployment/api deployment/agent
```

## Manual migration for vector index (run once after tables exist)
```sql
CREATE INDEX IF NOT EXISTS idx_endpoints_embedding_hnsw
  ON endpoints
  USING hnsw (embedding vector_cosine_ops)
  WITH (m = 16, ef_construction = 200);
```

## HPA scaling
- API: CPU at 70% → 3-10 pods
- Agent: CPU at 75% → 3-15 pods

## Backup schedule
- PG: incremental every 5min via pgBackRest to S3
- Redis: RDB dump every 6 hours to S3
