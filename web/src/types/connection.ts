export interface Connection {
  id: string;
  name: string;
  type: 'awx' | 'aap';
  role: 'source' | 'destination';
  scheme: 'http' | 'https';
  host: string;
  port: number;
  username: string;
  password: string;
  insecure: boolean;
  ping_status?: 'unknown' | 'ok' | 'error';
  ping_error?: string;
  auth_status?: 'unknown' | 'ok' | 'error';
  auth_error?: string;
  last_checked?: string;
}

export interface TestResult {
  ok: boolean;
  error?: string;
}
