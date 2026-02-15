import { useState, useEffect, useCallback } from 'react';
import {
  Button,
  Title,
  TextContent,
  Text,
  Alert,
  Label,
  Split,
  SplitItem,
  Flex,
  FlexItem,
  Card,
  CardBody,
  FormGroup,
  FormSelect,
  FormSelectOption,
} from '@patternfly/react-core';
import TimesIcon from '@patternfly/react-icons/dist/esm/icons/times-icon';
import { api } from '../api/client';
import { LogViewer } from '../components/LogViewer';
import { MigrationPreview } from '../components/MigrationPreview';
import type { Connection } from '../types/connection';
import type { MigrationPreviewData } from '../types/resources';

type Step = 'select' | 'preview' | 'run';

export function Migrate() {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [sourceId, setSourceId] = useState('');
  const [destId, setDestId] = useState('');
  const [step, setStep] = useState<Step>('select');
  const [previewJobId, setPreviewJobId] = useState('');
  const [runJobId, setRunJobId] = useState('');
  const [previewData, setPreviewData] = useState<MigrationPreviewData | null>(null);
  const [previewError, setPreviewError] = useState('');
  const [loading, setLoading] = useState(false);

  const loadConnections = useCallback(async () => {
    const conns = await api.listConnections() as Connection[];
    setConnections(conns);
  }, []);

  useEffect(() => { loadConnections(); }, [loadConnections]);

  const handlePreview = async () => {
    if (!sourceId || !destId) return;
    if (sourceId === destId) return;

    setLoading(true);
    setPreviewData(null);
    setPreviewError('');

    try {
      const result = await api.migrationPreview(sourceId, destId);
      setPreviewJobId(result.job_id);
      setStep('preview');
      // Poll for preview result
      pollPreview(result.job_id);
    } catch (err) {
      setPreviewError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  };

  const pollPreview = async (jobId: string) => {
    const poll = async () => {
      try {
        const resp = await api.getMigrationPreview(jobId) as Record<string, unknown>;
        if (resp.status === 'running') {
          setTimeout(poll, 1500);
          return;
        }
        if (resp.status === 'failed') {
          setPreviewError(resp.error as string || 'Preview failed');
          return;
        }
        // Successful preview — resp is MigrationPreviewData
        setPreviewData(resp as unknown as MigrationPreviewData);
      } catch {
        setTimeout(poll, 1500);
      }
    };
    // Start polling after a short delay to let the job start
    setTimeout(poll, 2000);
  };

  const handleRun = async () => {
    if (!previewJobId) return;
    setLoading(true);
    try {
      const result = await api.migrationRun(sourceId, destId, previewJobId);
      setRunJobId(result.job_id);
      setStep('run');
    } catch (err) {
      setPreviewError(err instanceof Error ? err.message : String(err));
    } finally {
      setLoading(false);
    }
  };

  const handleBack = () => {
    setStep('select');
    setPreviewJobId('');
    setRunJobId('');
    setPreviewData(null);
    setPreviewError('');
  };

  const sourceConn = connections.find(c => c.id === sourceId);
  const destConn = connections.find(c => c.id === destId);

  return (
    <>
      <Title headingLevel="h1" size="2xl">Migrate</Title>
      <TextContent style={{ marginBottom: 16 }}>
        <Text>Migrate resources from a source AWX/AAP platform to a destination AAP platform.</Text>
      </TextContent>

      {connections.length < 2 && (
        <Alert variant="info" isInline title="You need at least 2 connections configured to perform a migration." />
      )}

      {/* Step 1: Select source and destination */}
      {step === 'select' && (
        <Card>
          <CardBody>
            <Flex direction={{ default: 'column' }} spaceItems={{ default: 'spaceItemsMd' }}>
              <FlexItem>
                <FormGroup label="Source" fieldId="source-select">
                  <FormSelect
                    id="source-select"
                    value={sourceId}
                    onChange={(_e, val) => setSourceId(val)}
                    aria-label="Select source connection"
                  >
                    <FormSelectOption key="" value="" label="-- Select source --" isDisabled />
                    {connections.map(c => (
                      <FormSelectOption
                        key={c.id}
                        value={c.id}
                        label={`${c.name} (${c.type.toUpperCase()} — ${c.scheme}://${c.host}:${c.port})`}
                        isDisabled={c.id === destId}
                      />
                    ))}
                  </FormSelect>
                </FormGroup>
              </FlexItem>
              <FlexItem>
                <FormGroup label="Destination" fieldId="dest-select">
                  <FormSelect
                    id="dest-select"
                    value={destId}
                    onChange={(_e, val) => setDestId(val)}
                    aria-label="Select destination connection"
                  >
                    <FormSelectOption key="" value="" label="-- Select destination --" isDisabled />
                    {connections.map(c => (
                      <FormSelectOption
                        key={c.id}
                        value={c.id}
                        label={`${c.name} (${c.type.toUpperCase()} — ${c.scheme}://${c.host}:${c.port})`}
                        isDisabled={c.id === sourceId}
                      />
                    ))}
                  </FormSelect>
                </FormGroup>
              </FlexItem>
              {sourceId && destId && sourceId === destId && (
                <FlexItem>
                  <Alert variant="danger" isInline title="Source and destination cannot be the same connection." />
                </FlexItem>
              )}
              <FlexItem>
                <Button
                  variant="primary"
                  onClick={handlePreview}
                  isDisabled={!sourceId || !destId || sourceId === destId || loading}
                  isLoading={loading}
                >
                  Preview Migration
                </Button>
              </FlexItem>
            </Flex>
          </CardBody>
        </Card>
      )}

      {/* Step 2: Preview */}
      {step === 'preview' && (
        <>
          <Card style={{ marginBottom: 16 }}>
            <CardBody>
              <Split hasGutter>
                <SplitItem>
                  <Label color="blue" isCompact>Source</Label>{' '}
                  {sourceConn?.name} ({sourceConn?.type.toUpperCase()})
                </SplitItem>
                <SplitItem>→</SplitItem>
                <SplitItem>
                  <Label color="purple" isCompact>Destination</Label>{' '}
                  {destConn?.name} ({destConn?.type.toUpperCase()})
                </SplitItem>
              </Split>
            </CardBody>
          </Card>

          {previewJobId && (
            <div style={{ marginBottom: 16 }}>
              <Split hasGutter>
                <SplitItem isFilled>
                  <Title headingLevel="h3">Preview Log</Title>
                </SplitItem>
              </Split>
              <LogViewer jobId={previewJobId} />
            </div>
          )}

          {previewError && (
            <Alert variant="danger" isInline title={previewError} style={{ marginBottom: 16 }} />
          )}

          {previewData && (
            <div style={{ marginBottom: 16 }}>
              <Title headingLevel="h3" style={{ marginBottom: 8 }}>Preview Results</Title>
              <MigrationPreview preview={previewData} />
            </div>
          )}

          <Flex>
            <FlexItem>
              <Button variant="secondary" onClick={handleBack}>Back</Button>
            </FlexItem>
            {previewData && (
              <FlexItem>
                <Button
                  variant="primary"
                  onClick={handleRun}
                  isDisabled={loading}
                  isLoading={loading}
                >
                  Start Migration
                </Button>
              </FlexItem>
            )}
          </Flex>
        </>
      )}

      {/* Step 3: Run */}
      {step === 'run' && (
        <>
          <Card style={{ marginBottom: 16 }}>
            <CardBody>
              <Split hasGutter>
                <SplitItem>
                  <Label color="blue" isCompact>Source</Label>{' '}
                  {sourceConn?.name}
                </SplitItem>
                <SplitItem>→</SplitItem>
                <SplitItem>
                  <Label color="purple" isCompact>Destination</Label>{' '}
                  {destConn?.name}
                </SplitItem>
              </Split>
            </CardBody>
          </Card>

          {runJobId && (
            <div style={{ marginBottom: 16 }}>
              <Split hasGutter>
                <SplitItem isFilled>
                  <Title headingLevel="h3">Migration Log</Title>
                </SplitItem>
                <SplitItem>
                  <Button variant="plain" aria-label="Back" onClick={handleBack}>
                    <TimesIcon />
                  </Button>
                </SplitItem>
              </Split>
              <LogViewer jobId={runJobId} />
            </div>
          )}

          <Button variant="secondary" onClick={handleBack} style={{ marginTop: 16 }}>
            New Migration
          </Button>
        </>
      )}
    </>
  );
}
