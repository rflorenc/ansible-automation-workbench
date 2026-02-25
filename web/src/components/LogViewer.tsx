import { useEffect, useRef, useState } from 'react';
import { createJobLogSocket } from '../api/client';

interface Props {
  jobId: string;
  onClose?: (status: string) => void;
}

export function LogViewer({ jobId, onClose }: Props) {
  const [lines, setLines] = useState<string[]>([]);
  const [status, setStatus] = useState<string>('connecting');
  const bottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    setLines([]);
    setStatus('connecting');

    const ws = createJobLogSocket(
      jobId,
      (line) => {
        setLines(prev => [...prev, line]);
        setStatus('streaming');
      },
      (reason) => {
        setStatus(reason || 'closed');
        onClose?.(reason || 'closed');
      }
    );

    return () => ws.close();
  }, [jobId]);

  useEffect(() => {
    bottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [lines]);

  return (
    <div>
      <div style={{ marginBottom: '8px', fontSize: '0.85em', color: '#6a6e73' }}>
        Status: {status} | Lines: {lines.length}
      </div>
      <div
        style={{
          background: '#1e1e1e',
          color: '#d4d4d4',
          padding: '12px',
          borderRadius: '4px',
          fontFamily: '"Red Hat Mono", "Liberation Mono", monospace',
          fontSize: '13px',
          lineHeight: '1.5',
          maxHeight: '600px',
          overflow: 'auto',
          whiteSpace: 'pre-wrap',
          wordBreak: 'break-all',
        }}
      >
        {lines.map((line, i) => (
          <div key={i}>{line}</div>
        ))}
        <div ref={bottomRef} />
      </div>
    </div>
  );
}
