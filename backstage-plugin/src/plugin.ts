import {
  createApiFactory,
  createComponentExtension,
  createPlugin,
  createRoutableExtension,
  createRouteRef,
  discoveryApiRef,
  fetchApiRef,
} from '@backstage/core-plugin-api';
import { K8sAdvisorClient, k8sAdvisorApiRef } from './api';

export const rootRouteRef = createRouteRef({
  id: 'k8s-advisor',
});

export const k8sAdvisorPlugin = createPlugin({
  id: 'k8s-advisor',
  apis: [
    createApiFactory({
      api: k8sAdvisorApiRef,
      deps: { discoveryApi: discoveryApiRef, fetchApi: fetchApiRef },
      factory: ({ discoveryApi, fetchApi }) =>
        new K8sAdvisorClient({ discoveryApi, fetchApi }),
    }),
  ],
  routes: {
    root: rootRouteRef,
  },
});

export const K8sAdvisorPage = k8sAdvisorPlugin.provide(
  createRoutableExtension({
    name: 'K8sAdvisorPage',
    component: () =>
      import('./components/K8sAdvisorPage').then(m => m.K8sAdvisorPage),
    mountPoint: rootRouteRef,
  }),
);

export const EntityK8sAdvisorCard = k8sAdvisorPlugin.provide(
  createComponentExtension({
    name: 'EntityK8sAdvisorCard',
    component: {
      lazy: () =>
        import('./components/EntityK8sAdvisorCard').then(
          m => m.EntityK8sAdvisorCard,
        ),
    },
  }),
);
