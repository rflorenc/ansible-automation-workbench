import { useState } from 'react';
import {
  Modal,
  ModalVariant,
  Form,
  FormGroup,
  FormHelperText,
  HelperText,
  HelperTextItem,
  TextInput,
  FormSelect,
  FormSelectOption,
  Checkbox,
  Button,
} from '@patternfly/react-core';
import type { Connection } from '../types/connection';

interface Props {
  isOpen: boolean;
  initial?: Partial<Connection>;
  onSave: (conn: Omit<Connection, 'id'>) => void;
  onClose: () => void;
}

export function ConnectionForm({ isOpen, initial, onSave, onClose }: Props) {
  const [name, setName] = useState(initial?.name || '');
  const [type, setType] = useState<'awx' | 'aap'>(initial?.type || 'awx');
  const [role, setRole] = useState<'source' | 'destination'>(initial?.role || 'source');
  const [scheme, setScheme] = useState<'http' | 'https'>(initial?.scheme || 'http');
  const [host, setHost] = useState(initial?.host || '');
  const [port, setPort] = useState(initial?.port || 80);
  const [username, setUsername] = useState(initial?.username || 'admin');
  const [password, setPassword] = useState(initial?.password || '');
  const [insecure, setInsecure] = useState(initial?.insecure ?? true);

  const handleSubmit = () => {
    onSave({ name, type, role, scheme, host, port, username, password, insecure });
  };

  return (
    <Modal
      isOpen={isOpen}
      onClose={onClose}
      variant={ModalVariant.medium}
      title={initial?.name ? 'Edit Connection' : 'Add Connection'}
      actions={[
        <Button key="save" variant="primary" onClick={handleSubmit}>Save</Button>,
        <Button key="cancel" variant="link" onClick={onClose}>Cancel</Button>,
      ]}
    >
      <Form isHorizontal>
        <FormGroup label="Name" isRequired fieldId="name">
          <TextInput id="name" value={name} onChange={(_e, v) => setName(v)} placeholder="Lab AWX" />
        </FormGroup>
        <FormGroup label="Type" fieldId="type">
          <FormSelect id="type" value={type} onChange={(_e, v) => {
            const t = v as 'awx' | 'aap';
            setType(t);
            if (t === 'aap') {
              setScheme('https');
              setPort(443);
              setRole('destination');
            } else {
              setScheme('http');
              setPort(80);
              setRole('source');
            }
          }}>
            <FormSelectOption value="awx" label="AWX" />
            <FormSelectOption value="aap" label="AAP" />
          </FormSelect>
        </FormGroup>
        <FormGroup label="Role" fieldId="role">
          <FormSelect id="role" value={role} onChange={(_e, v) => setRole(v as 'source' | 'destination')}
            isDisabled={type === 'awx'}
          >
            <FormSelectOption value="source" label="Source (migrate FROM)" />
            <FormSelectOption value="destination" label="Destination (migrate TO)" />
          </FormSelect>
          <FormHelperText>
            <HelperText>
              <HelperTextItem>
                {type === 'awx' ? 'AWX instances are always sources' : 'AAP can be source (older) or destination (2.5+)'}
              </HelperTextItem>
            </HelperText>
          </FormHelperText>
        </FormGroup>
        <FormGroup label="Scheme" fieldId="scheme">
          <FormSelect id="scheme" value={scheme} onChange={(_e, v) => {
            setScheme(v as 'http' | 'https');
            if (v === 'https' && port === 80) setPort(443);
            if (v === 'http' && port === 443) setPort(80);
          }}>
            <FormSelectOption value="http" label="HTTP" />
            <FormSelectOption value="https" label="HTTPS" />
          </FormSelect>
        </FormGroup>
        <FormGroup label="Host" isRequired fieldId="host">
          <TextInput id="host" value={host} onChange={(_e, v) => setHost(v)} placeholder="awx.lab.local" />
        </FormGroup>
        <FormGroup label="Port" fieldId="port">
          <TextInput id="port" type="number" value={port} onChange={(_e, v) => setPort(Number(v))} />
        </FormGroup>
        <FormGroup label="Username" fieldId="username">
          <TextInput id="username" value={username} onChange={(_e, v) => setUsername(v)} />
        </FormGroup>
        <FormGroup label="Password" fieldId="password">
          <TextInput id="password" type="password" value={password} onChange={(_e, v) => setPassword(v)} />
        </FormGroup>
        <FormGroup fieldId="insecure">
          <Checkbox
            id="insecure"
            label="Skip TLS verification"
            isChecked={insecure}
            onChange={(_e, v) => setInsecure(v)}
          />
        </FormGroup>
      </Form>
    </Modal>
  );
}
