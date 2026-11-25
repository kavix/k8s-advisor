import React, { useEffect, useState } from 'react';
import { InfoCard, ResponseErrorPanel } from '@backstage/core-components';
import { useApi } from '@backstage/core-plugin-api';
import { useEntity } from '@backstage/plugin-catalog-react';
import { Advisory, k8sAdvisorApiRef } from '../api';
import { AdvisoriesTable } from './AdvisoriesTable';

export const EntityK8sAdvisorCard = () => {
  const { entity } = useEntity();
  const api = useApi(k8sAdvisorApiRef);
  const namespace =
    entity.metadata.annotations?.['backstage.io/kubernetes-namespace'] ??
    entity.metadata.annotations?.['kubernetes.io/namespace'];

  const [advisories, setAdvisories] = useState<Advisory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | undefined>();

  useEffect(() => {
    let mounted = true;
    setLoading(true);
    api
      .listAdvisories({ namespace, limit: 25 })
      .then(items => {
        if (mounted) setAdvisories(items);
      })
      .catch(err => {
        if (mounted) setError(err as Error);
      })
      .finally(() => {
        if (mounted) setLoading(false);
      });
    return () => {
      mounted = false;
    };
  }, [api, namespace]);

  return (
    <InfoCard
      title="K8s Advisor Recommendations"
      subheader={namespace ? `Namespace: ${namespace}` : 'All namespaces'}
    >
      {error ? (
        <ResponseErrorPanel error={error} />
      ) : (
        <AdvisoriesTable advisories={advisories} loading={loading} />
      )}
    </InfoCard>
  );
};
