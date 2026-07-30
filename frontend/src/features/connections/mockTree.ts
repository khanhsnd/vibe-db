import type { TreeNode } from './tree'

export const MOCK_TREE: TreeNode[] = [
  {
    id: 'conn-local',
    kind: 'connection',
    label: 'localhost',
    status: 'connected',
    children: [
      {
        id: 'db-shop',
        kind: 'database',
        label: 'shop',
        children: [
          {
            id: 'db-shop-tables',
            kind: 'folder',
            label: 'Tables',
            children: [
              { id: 't-users', kind: 'table', label: 'users', badge: '1.2k' },
              { id: 't-orders', kind: 'table', label: 'orders', badge: '18.4k' },
              { id: 't-order-items', kind: 'table', label: 'order_items', badge: '52.1k' },
              { id: 't-products', kind: 'table', label: 'products', badge: '840' },
              { id: 't-categories', kind: 'table', label: 'categories', badge: '26' },
            ],
          },
          {
            id: 'db-shop-views',
            kind: 'folder',
            label: 'Views',
            children: [{ id: 'v-revenue', kind: 'view', label: 'monthly_revenue' }],
          },
          { id: 'db-shop-procs', kind: 'folder', label: 'Procedures', children: [] },
          { id: 'db-shop-funcs', kind: 'folder', label: 'Functions', children: [] },
        ],
      },
      { id: 'db-analytics', kind: 'database', label: 'analytics', children: [] },
      { id: 'db-mysql', kind: 'database', label: 'mysql', children: [] },
    ],
  },
  { id: 'conn-staging', kind: 'connection', label: 'staging', status: 'idle', children: [] },
  { id: 'conn-prod', kind: 'connection', label: 'prod-replica', status: 'error', children: [] },
]

export const MOCK_EXPANDED = new Set(['conn-local', 'db-shop', 'db-shop-tables'])
