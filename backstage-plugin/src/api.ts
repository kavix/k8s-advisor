import { createApiRef, DiscoveryApi, FetchApi } from '@backstage/core-plugin-api';

export interface ClassificationResult {
  issue_category: string;
  severity: 'low' | 'medium' | 'high' | 'critical';
  severity_score: number;
  immediate_action_probability: number;
  recommended_action: string;
  classification_source: 'jev' | 'rule_fallback';
}

export interface Advisory {
  created_at: string;
  cluster_name: string;
  namespace: string;
  resource_kind: string;
  resource_name: string;
  classification: ClassificationResult;
  action_summary: string;
  context?: Record<string, unknown>;
}

export interface ListAdvisoriesOptions {
  namespace?: string;
  cluster?: string;
  limit?: number;
}

export interface K8sAdvisorApi {
  listAdvisories(options?: ListAdvisoriesOptions): Promise<Advisory[]>;
}

export const k8sAdvisorApiRef = createApiRef<K8sAdvisorApi>({
  id: 'plugin.k8s-advisor.api',
});

export class K8sAdvisorClient implements K8sAdvisorApi {
  private readonly discoveryApi: DiscoveryApi;
  private readonly fetchApi: FetchApi;

  constructor(options: { discoveryApi: DiscoveryApi; fetchApi: FetchApi }) {
    this.discoveryApi = options.discoveryApi;
    this.fetchApi = options.fetchApi;
  }

  async listAdvisories(options: ListAdvisoriesOptions = {}): Promise<Advisory[]> {
    const proxyUrl = await this.discoveryApi.getBaseUrl('proxy');
    const params = new URLSearchParams();
    if (options.namespace) params.set('namespace', options.namespace);
    if (options.cluster) params.set('cluster', options.cluster);
    if (options.limit) params.set('limit', String(options.limit));

    const query = params.toString();
    const url = `${proxyUrl}/k8s-advisor/api/v1/advisories${query ? `?${query}` : ''}`;

    const resp = await this.fetchApi.fetch(url);
    if (!resp.ok) {
      throw new Error(`k8s-advisor API error (${resp.status}): ${resp.statusText}`);
    }
    const data = (await resp.json()) as { advisories?: Advisory[] };
    return data.advisories ?? [];
  }
}
