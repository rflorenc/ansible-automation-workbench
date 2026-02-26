const BASE = '';

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const opts: RequestInit = {
    method,
    headers: { 'Content-Type': 'application/json' },
  };
  if (body !== undefined) {
    opts.body = JSON.stringify(body);
  }
  const resp = await fetch(`${BASE}${path}`, opts);
  if (resp.status === 204) return undefined as T;
  const data = await resp.json();
  if (!resp.ok) throw new Error(data.error || `HTTP ${resp.status}`);
  return data as T;
}

export const api = {
  // Connections
  createConnection: (conn: unknown) => request<unknown>('POST', '/api/connections', conn),
  listConnections: () => request<unknown[]>('GET', '/api/connections'),
  updateConnection: (id: string, conn: unknown) => request<unknown>('PUT', `/api/connections/${id}`, conn),
  deleteConnection: (id: string) => request<void>('DELETE', `/api/connections/${id}`),
  testConnection: (id: string) => request<{ ok: boolean; error?: string }>('POST', `/api/connections/${id}/test`),

  // Resources
  listResourceTypes: (connId: string) => request<unknown[]>('GET', `/api/connections/${connId}/resources`),
  listResources: (connId: string, type: string) => request<unknown[]>('GET', `/api/connections/${connId}/resources/${type}`),

  // Operations
  runCleanup: (connId: string) => request<{ job_id: string }>('POST', `/api/connections/${connId}/cleanup`),
  runPopulate: (connId: string) => request<{ job_id: string }>('POST', `/api/connections/${connId}/populate`),
  runExport: (connId: string) => request<{ job_id: string; output_dir: string }>('POST', `/api/connections/${connId}/export`),

  // Migration
  migrationPreview: (sourceId: string, destinationId: string) =>
    request<{ job_id: string }>('POST', '/api/migrate/preview', { source_id: sourceId, destination_id: destinationId }),
  getMigrationPreview: (jobId: string) =>
    request<unknown>('GET', `/api/migrate/preview/${jobId}`),
  migrationRun: (sourceId: string, destinationId: string, previewJobId: string, exclude?: Record<string, string[]>) =>
    request<{ job_id: string }>('POST', '/api/migrate/run', {
      source_id: sourceId,
      destination_id: destinationId,
      preview_job_id: previewJobId,
      exclude: exclude || {},
    }),

  // Exclusions
  getExclusions: () => request<unknown>('GET', '/api/exclusions'),

  // Jobs
  listJobs: () => request<unknown[]>('GET', '/api/jobs'),
  getJob: (id: string) => request<unknown>('GET', `/api/jobs/${id}`),
  cancelJob: (jobId: string) => request<{ status: string }>('POST', `/api/jobs/${jobId}/cancel`),
};

export function createJobLogSocket(jobId: string, onMessage: (line: string) => void, onClose?: (status: string) => void): WebSocket {
  const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
  const ws = new WebSocket(`${proto}//${window.location.host}/ws/jobs/${jobId}/logs`);
  ws.onmessage = (e) => onMessage(e.data);
  ws.onclose = (e) => onClose?.(e.reason || 'closed');
  return ws;
}
