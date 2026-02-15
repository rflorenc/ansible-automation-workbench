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
}

export interface TestResult {
  ok: boolean;
  error?: string;
}
