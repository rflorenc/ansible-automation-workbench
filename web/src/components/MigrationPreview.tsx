import { useState } from 'react';
import {
  Alert,
  Label,
  ExpandableSection,
} from '@patternfly/react-core';
import type { MigrationPreviewData } from '../types/resources';

const resourceTypeLabels: Record<string, string> = {
  organizations: 'Organizations',
  teams: 'Teams',
  users: 'Users',
  credential_types: 'Credential Types',
  credentials: 'Credentials',
  projects: 'Projects',
  inventories: 'Inventories',
  job_templates: 'Job Templates',
  workflow_job_templates: 'Workflow Job Templates',
  schedules: 'Schedules',
};

const displayOrder = [
  'organizations', 'teams', 'users', 'credential_types', 'credentials',
  'projects', 'inventories', 'job_templates', 'workflow_job_templates', 'schedules',
];

interface Props {
  preview: MigrationPreviewData;
}

export function MigrationPreview({ preview }: Props) {
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});

  let createCount = 0;
  let skipCount = 0;
  for (const items of Object.values(preview.resources)) {
    for (const item of items) {
      if (item.action === 'create') createCount++;
      else skipCount++;
    }
  }

  const toggleExpanded = (type: string) => {
    setExpanded(prev => ({ ...prev, [type]: !prev[type] }));
  };

  const orderedTypes = displayOrder.filter(t => preview.resources[t]?.length > 0);

  return (
    <div>
      {preview.warnings?.map((w, i) => (
        <Alert key={i} variant="warning" isInline title={w} style={{ marginBottom: 8 }} />
      ))}

      <div style={{ margin: '16px 0', fontSize: '1.1em' }}>
        <strong>{createCount}</strong> to create, <strong>{skipCount}</strong> to skip (already exist)
      </div>

      {orderedTypes.map(type => {
        const items = preview.resources[type];
        const creates = items.filter(i => i.action === 'create').length;
        const skips = items.length - creates;
        const label = resourceTypeLabels[type] || type;

        return (
          <ExpandableSection
            key={type}
            toggleText={`${label} (${items.length}) — ${creates} create, ${skips} skip`}
            isExpanded={expanded[type] || false}
            onToggle={() => toggleExpanded(type)}
          >
            <table style={{ width: '100%', borderCollapse: 'collapse', fontSize: '0.9em' }}>
              <thead>
                <tr style={{ borderBottom: '1px solid #d2d2d2', textAlign: 'left' }}>
                  <th style={{ padding: '4px 8px' }}>Name</th>
                  <th style={{ padding: '4px 8px' }}>Action</th>
                  <th style={{ padding: '4px 8px' }}>Source ID</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item, i) => (
                  <tr key={i} style={{ borderBottom: '1px solid #eee' }}>
                    <td style={{ padding: '4px 8px' }}>{item.name}</td>
                    <td style={{ padding: '4px 8px' }}>
                      <Label
                        color={item.action === 'create' ? 'green' : 'grey'}
                        isCompact
                      >
                        {item.action === 'create' ? 'Create' : 'Skip (exists)'}
                      </Label>
                    </td>
                    <td style={{ padding: '4px 8px' }}>{item.source_id}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </ExpandableSection>
        );
      })}
    </div>
  );
}
