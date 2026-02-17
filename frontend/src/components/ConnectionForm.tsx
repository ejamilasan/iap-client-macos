import { useState } from 'react';
import { Connection, Instance, RDPSettings } from '../types';

interface ConnectionFormProps {
  instance?: Instance;
  connection?: Connection;
  onSave: (connection: Connection, password?: string, saveConnection?: boolean) => void;
  onCancel: () => void;
}

export function ConnectionForm({ instance, connection, onSave, onCancel }: ConnectionFormProps) {
  const [username, setUsername] = useState(connection?.username || '');
  const [password, setPassword] = useState('');
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [generatingPassword, setGeneratingPassword] = useState(false);
  const [showPassword, setShowPassword] = useState(false);
  const [showAdvanced, setShowAdvanced] = useState(false);
  const [name, setName] = useState(connection?.name || instance?.name || '');
  const [domain, setDomain] = useState(connection?.domain || '');
  const [rdpSettings, setRdpSettings] = useState<RDPSettings>(
    connection?.rdpSettings || {
      fullScreen: false,
      screenWidth: 1920,
      screenHeight: 1080,
      colorDepth: 32,
      audioMode: 0,
      driveRedirect: false,
      clipboardShare: true,
    }
  );

  const instanceName = instance?.name || connection?.instance || '';
  const projectId = instance?.project || connection?.project || '';
  const zone = instance?.zone || connection?.zone || '';

  const handleGeneratePassword = async () => {
    if (!username.trim()) {
      setError('Please enter a username first');
      return;
    }

    setGeneratingPassword(true);
    setError(null);

    try {
      // @ts-ignore - Wails binding
      const newPassword = await window.go.main.App.ResetWindowsPassword(
        projectId,
        zone,
        instanceName,
        username
      );
      setPassword(newPassword);
    } catch (err: any) {
      setError(`Failed to generate password: ${err.message || err}`);
    } finally {
      setGeneratingPassword(false);
    }
  };

  const handleConnect = async (saveConnection: boolean) => {
    if (!username.trim() || !password.trim()) {
      setError('Username and password are required');
      return;
    }

    setSaving(true);
    setError(null);

    try {
      const connData: Connection = {
        id: connection?.id || '',
        name: name || instanceName,
        project: projectId,
        zone: zone,
        instance: instanceName,
        username,
        domain,
        createdAt: connection?.createdAt || new Date().toISOString(),
        updatedAt: new Date().toISOString(),
        rdpSettings,
      };

      // Save password to keychain
      try {
        // @ts-ignore - Wails binding
        await window.go.main.App.StorePassword(instanceName, username, password);
      } catch (err) {
        console.error('Failed to save password:', err);
      }

      onSave(connData, password, saveConnection);
    } catch (err) {
      setError('Failed to connect');
      console.error('Failed to connect:', err);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="connection-form compact">
      <h2>Connect to {instanceName}</h2>
      <p className="form-subtitle">{projectId} / {zone}</p>

      {error && <div className="error-message">{error}</div>}

      <div className="form-group">
        <label htmlFor="username">Username</label>
        <input
          id="username"
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          placeholder="Enter username"
          autoComplete="off"
          autoCorrect="off"
          autoCapitalize="off"
          spellCheck={false}
          autoFocus
        />
      </div>

      <div className="form-group">
        <label htmlFor="password">Password</label>
        <div className="password-input-row">
          <input
            id="password"
            type={showPassword ? 'text' : 'password'}
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            placeholder="Enter password"
          />
          <button
            type="button"
            onClick={() => setShowPassword(!showPassword)}
            className="btn btn-secondary btn-sm btn-show-password"
            title={showPassword ? 'Hide password' : 'Show password'}
          >
            {showPassword ? 'Hide' : 'Show'}
          </button>
          <button
            type="button"
            onClick={handleGeneratePassword}
            disabled={generatingPassword || !username.trim()}
            className="btn btn-secondary btn-sm"
            title="Generate new Windows password"
          >
            {generatingPassword ? 'Generating...' : 'Generate'}
          </button>
        </div>
      </div>

      <div className="advanced-toggle" onClick={() => setShowAdvanced(!showAdvanced)}>
        <span className="toggle-icon">{showAdvanced ? '▼' : '▶'}</span>
        <span>Advanced options</span>
      </div>

      {showAdvanced && (
        <div className="advanced-options">
          <div className="form-group">
            <label htmlFor="name">Save as</label>
            <input
              id="name"
              type="text"
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder={instanceName}
            />
          </div>

          <div className="form-group">
            <label htmlFor="domain">Domain</label>
            <input
              id="domain"
              type="text"
              value={domain}
              onChange={(e) => setDomain(e.target.value)}
              placeholder="Optional"
            />
          </div>

          <div className="form-group">
            <label htmlFor="resolution">Resolution</label>
            <select
              id="resolution"
              value={`${rdpSettings.screenWidth}x${rdpSettings.screenHeight}`}
              onChange={(e) => {
                const [w, h] = e.target.value.split('x').map(Number);
                setRdpSettings({...rdpSettings, screenWidth: w, screenHeight: h});
              }}
            >
              <option value="1024x768">1024 x 768</option>
              <option value="1280x720">1280 x 720 (HD)</option>
              <option value="1280x800">1280 x 800</option>
              <option value="1366x768">1366 x 768</option>
              <option value="1440x900">1440 x 900</option>
              <option value="1600x900">1600 x 900</option>
              <option value="1920x1080">1920 x 1080 (Full HD)</option>
              <option value="2560x1440">2560 x 1440 (QHD)</option>
              <option value="3840x2160">3840 x 2160 (4K)</option>
            </select>
          </div>
        </div>
      )}

      <div className="form-actions">
        <button type="button" onClick={onCancel} className="btn btn-secondary">
          Cancel
        </button>
        <button
          type="button"
          onClick={() => handleConnect(false)}
          disabled={saving || !username.trim() || !password.trim()}
          className="btn btn-secondary"
        >
          Connect
        </button>
        <button
          type="button"
          onClick={() => handleConnect(true)}
          disabled={saving || !username.trim() || !password.trim()}
          className="btn btn-primary"
        >
          {saving ? 'Connecting...' : 'Save & Connect'}
        </button>
      </div>
    </div>
  );
}
