import { useState, useEffect, useCallback } from 'react';
import {
  Button,
  Card,
  CardBody,
  CardFooter,
  CardHeader,
  CardTitle,
  Title,
  TextContent,
  Text,
  Gallery,
  Label,
  Split,
  SplitItem,
  DescriptionList,
  DescriptionListGroup,
  DescriptionListTerm,
  DescriptionListDescription,
  Alert,
} from '@patternfly/react-core';
import { Dropdown, DropdownItem, KebabToggle } from '@patternfly/react-core/deprecated';
import { api } from '../api/client';
import { ConnectionForm } from '../components/ConnectionForm';
import type { Connection, TestResult } from '../types/connection';

export function Dashboard() {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [showForm, setShowForm] = useState(false);
  const [editConn, setEditConn] = useState<Connection | null>(null);
  const [testResults, setTestResults] = useState<Record<string, TestResult>>({});
  const [openMenu, setOpenMenu] = useState<string | null>(null);

  const loadConnections = useCallback(async () => {
    const conns = await api.listConnections() as Connection[];
    setConnections(conns);
  }, []);

  useEffect(() => { loadConnections(); }, [loadConnections]);

  const handleSave = async (conn: Omit<Connection, 'id'>) => {
    if (editConn) {
      await api.updateConnection(editConn.id, conn);
    } else {
      await api.createConnection(conn);
    }
    setShowForm(false);
    setEditConn(null);
    loadConnections();
  };

  const handleDelete = async (id: string) => {
    await api.deleteConnection(id);
    loadConnections();
  };

  const handleTest = async (id: string) => {
    const result = await api.testConnection(id);
    setTestResults(prev => ({ ...prev, [id]: result }));
  };

  const dropdownItems = (conn: Connection) => [
    <DropdownItem key="edit" onClick={() => { setEditConn(conn); setShowForm(true); }}>Edit</DropdownItem>,
    <DropdownItem key="delete" onClick={() => handleDelete(conn.id)} style={{ color: '#c9190b' }}>Delete</DropdownItem>,
  ];

  const sources = connections.filter(c => c.role === 'source');
  const destinations = connections.filter(c => c.role === 'destination');

  const renderCard = (conn: Connection) => {
    const testResult = testResults[conn.id];
    return (
      <Card key={conn.id}>
        <CardHeader
          actions={{
            actions: (
              <Dropdown
                isOpen={openMenu === conn.id}
                onSelect={() => setOpenMenu(null)}
                toggle={<KebabToggle onToggle={(_e, open) => setOpenMenu(open ? conn.id : null)} />}
                isPlain
                dropdownItems={dropdownItems(conn)}
                position="right"
              />
            ),
          }}
        >
          <CardTitle>
            <Split hasGutter>
              <SplitItem>{conn.name}</SplitItem>
              <SplitItem>
                <Label color={conn.type === 'awx' ? 'blue' : 'purple'}>{conn.type.toUpperCase()}</Label>
              </SplitItem>
              {testResult && (
                <SplitItem>
                  <Label color={testResult.ok ? 'green' : 'red'}>
                    {testResult.ok ? 'Connected' : 'Failed'}
                  </Label>
                </SplitItem>
              )}
            </Split>
          </CardTitle>
        </CardHeader>
        <CardBody>
          <DescriptionList isHorizontal isCompact>
            <DescriptionListGroup>
              <DescriptionListTerm>URL</DescriptionListTerm>
              <DescriptionListDescription>
                {conn.scheme}://{conn.host}:{conn.port}
              </DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>User</DescriptionListTerm>
              <DescriptionListDescription>{conn.username}</DescriptionListDescription>
            </DescriptionListGroup>
            <DescriptionListGroup>
              <DescriptionListTerm>TLS Verify</DescriptionListTerm>
              <DescriptionListDescription>{conn.insecure ? 'Off' : 'On'}</DescriptionListDescription>
            </DescriptionListGroup>
          </DescriptionList>
          {testResult && !testResult.ok && (
            <Alert variant="danger" isInline isPlain title={testResult.error} style={{ marginTop: 8 }} />
          )}
        </CardBody>
        <CardFooter>
          <Button variant="secondary" size="sm" onClick={() => handleTest(conn.id)}>Test</Button>
        </CardFooter>
      </Card>
    );
  };

  return (
    <>
      <Title headingLevel="h1" size="2xl">Connections</Title>
      <TextContent style={{ marginBottom: 16 }}>
        <Text>Manage your AWX and AAP connections.</Text>
      </TextContent>
      <Button variant="primary" onClick={() => { setEditConn(null); setShowForm(true); }} style={{ marginBottom: 16 }}>
        Add Connection
      </Button>

      {connections.length === 0 && (
        <Alert variant="info" isInline title="No connections yet. Click 'Add Connection' to get started." />
      )}

      {sources.length > 0 && (
        <>
          <Title headingLevel="h2" size="xl" style={{ marginTop: 16, marginBottom: 8 }}>Sources</Title>
          <Gallery hasGutter minWidths={{ default: '350px' }}>
            {sources.map(conn => renderCard(conn))}
          </Gallery>
        </>
      )}

      {destinations.length > 0 && (
        <>
          <Title headingLevel="h2" size="xl" style={{ marginTop: 24, marginBottom: 8 }}>Destinations</Title>
          <Gallery hasGutter minWidths={{ default: '350px' }}>
            {destinations.map(conn => renderCard(conn))}
          </Gallery>
        </>
      )}

      <ConnectionForm
        isOpen={showForm}
        initial={editConn || undefined}
        onSave={handleSave}
        onClose={() => { setShowForm(false); setEditConn(null); }}
      />
    </>
  );
}
