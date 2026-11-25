import React from 'react';
import {
  StatusAborted,
  StatusError,
  StatusOK,
  StatusWarning,
  Table,
  TableColumn,
} from '@backstage/core-components';
import { Advisory } from '../api';

function SeverityBadge({ severity }: { severity: string }) {
  switch (severity) {
    case 'critical':
      return <StatusError>Critical</StatusError>;
    case 'high':
      return <StatusAborted>High</StatusAborted>;
    case 'medium':
      return <StatusWarning>Medium</StatusWarning>;
    default:
      return <StatusOK>Low</StatusOK>;
  }
}

export const AdvisoriesTable = ({
  advisories,
  loading,
}: {
  advisories: Advisory[];
  loading: boolean;
}) => {
  const columns: TableColumn<Advisory>[] = [
    {
      title: 'Severity',
      field: 'classification.severity',
      render: row => <SeverityBadge severity={row.classification.severity} />,
    },
    {
      title: 'Resource',
      field: 'resource_name',
      render: row => `${row.namespace}/${row.resource_kind}/${row.resource_name}`,
    },
    {
      title: 'Category',
      field: 'classification.issue_category',
    },
    {
      title: 'Recommended Action',
      field: 'action_summary',
    },
    {
      title: 'Action Prob.',
      field: 'classification.immediate_action_probability',
      render: row =>
        `${Math.round(row.classification.immediate_action_probability * 100)}%`,
    },
    {
      title: 'Source',
      field: 'classification.classification_source',
    },
    {
      title: 'Time',
      field: 'created_at',
      render: row => new Date(row.created_at).toLocaleString(),
    },
  ];

  return (
    <Table
      title="Kubernetes Operational Advisories"
      options={{ search: true, paging: true, pageSize: 10 }}
      columns={columns}
      data={advisories}
      isLoading={loading}
    />
  );
};
