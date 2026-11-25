import React, { useEffect, useState } from 'react';
import {
  Content,
  Header,
  Page,
  ResponseErrorPanel,
} from '@backstage/core-components';
import { useApi } from '@backstage/core-plugin-api';
import { Button } from '@material-ui/core';
import RefreshIcon from '@material-ui/icons/Refresh';
import { Advisory, k8sAdvisorApiRef } from '../api';
import { AdvisoriesTable } from './AdvisoriesTable';

export const K8sAdvisorPage = () => {
  const api = useApi(k8sAdvisorApiRef);
  const [advisories, setAdvisories] = useState<Advisory[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<Error | undefined>();

  const load = async () => {
    setLoading(true);
    setError(undefined);
    try {
      const items = await api.listAdvisories({ limit: 100 });
      setAdvisories(items);
    } catch (err) {
      setError(err as Error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    void load();
  }, []);

  return (
    <Page themeId="tool">
      <Header
        title="Kubernetes Advisor"
        subtitle="Real-time cluster diagnostics powered by TimescaleDB & Jev"
      >
        <Button
          variant="contained"
          color="primary"
          startIcon={<RefreshIcon />}
          onClick={() => void load()}
        >
          Refresh
        </Button>
      </Header>
      <Content>
        {error ? (
          <ResponseErrorPanel error={error} />
        ) : (
          <AdvisoriesTable advisories={advisories} loading={loading} />
        )}
      </Content>
    </Page>
  );
};
