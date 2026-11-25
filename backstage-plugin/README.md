# Backstage Plugin for `k8s-advisor`

Provides:
- **`K8sAdvisorPage`**: Full-page cluster-wide operational advisories view.
- **`EntityK8sAdvisorCard`**: Entity-scoped card filtered by `backstage.io/kubernetes-namespace` (or `kubernetes.io/namespace`) annotation.

## Setup in Backstage

1. **Configure Proxy** in `app-config.yaml`:

```yaml
proxy:
  endpoints:
    /k8s-advisor:
      target: http://localhost:8080
      changeOrigin: true
```

2. **Add Route** in `packages/app/src/App.tsx`:

```tsx
import { K8sAdvisorPage } from '@kavix/backstage-plugin-k8s-advisor';

<Route path="/k8s-advisor" element={<K8sAdvisorPage />} />
```

3. **Add Entity Card** in `packages/app/src/components/catalog/EntityPage.tsx`:

```tsx
import { EntityK8sAdvisorCard } from '@kavix/backstage-plugin-k8s-advisor';

<EntityK8sAdvisorCard />
```
