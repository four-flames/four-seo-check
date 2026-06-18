# DevOps

Infrastructure configuration for four-seo-check.

## Structure (planned)

```
devops/
├── k8s/           # Kubernetes manifests (Deployments, CronJobs, Services)
├── argocd/        # ArgoCD application definitions
├── dagu/          # Dagu DAG definitions for scheduled crawls
├── base/          # Base configurations
└── overlays/      # Environment-specific overlays (dev, staging, prod)
```

## Status

Phase 1: Placeholder. Infrastructure manifests will be added during later phases.
