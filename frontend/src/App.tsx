import { useState, useEffect, useCallback } from 'react';
import './App.css';
import { AuthStatusComponent } from './components/AuthStatus';
import { Sidebar } from './components/Sidebar';
import { SessionTabs } from './components/SessionTabs';
import { SessionViewer } from './components/SessionViewer';
import { ConnectionForm } from './components/ConnectionForm';
import { Instance, Connection, ViewerSession } from './types';

type Modal = 'none' | 'connection-form';

function App() {
  const [authenticated, setAuthenticated] = useState(false);
  const [modal, setModal] = useState<Modal>('none');
  const [selectedInstance, setSelectedInstance] = useState<Instance | null>(null);
  const [editingConnection, setEditingConnection] = useState<Connection | null>(null);
  const [showWindowsOnly, setShowWindowsOnly] = useState(true);
  const [rdpInstalled, setRdpInstalled] = useState<boolean | null>(null);

  // Multi-session state
  const [sessions, setSessions] = useState<ViewerSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);

  // Theme state
  const [theme, setTheme] = useState<'light' | 'dark'>(() => {
    const saved = localStorage.getItem('theme');
    return (saved as 'light' | 'dark') || 'light';
  });

  useEffect(() => {
    document.documentElement.setAttribute('data-theme', theme);
    localStorage.setItem('theme', theme);
  }, [theme]);

  const toggleTheme = () => {
    setTheme(prev => prev === 'light' ? 'dark' : 'light');
  };

  useEffect(() => {
    checkRdpInstalled();
  }, []);

  const checkRdpInstalled = async () => {
    try {
      // @ts-ignore - Wails binding
      const installed = await window.go.main.App.IsRDPClientInstalled();
      setRdpInstalled(installed);
    } catch (error) {
      console.error('Failed to check RDP client:', error);
    }
  };

  // Handle connecting to an instance from the sidebar
  const handleInstanceSelect = (instance: Instance) => {
    setSelectedInstance(instance);
    setEditingConnection(null);
    setModal('connection-form');
  };

  // Handle connecting to a saved connection
  const handleConnectSaved = async (connection: Connection, password?: string) => {
    if (!password) {
      // Need password for embedded viewer - prompt via connection form
      setEditingConnection(connection);
      setSelectedInstance(null);
      setModal('connection-form');
      return;
    }

    await startSession(connection.project, connection.zone, connection.instance, connection.username, password);
  };

  // Start a new RDP session
  const startSession = async (project: string, zone: string, instance: string, username: string, password: string) => {
    try {
      // Start tunnel
      // @ts-ignore - Wails binding
      const tunnelInfo = await window.go.main.App.StartTunnel(project, zone, instance);

      // Create session
      const sessionId = `session-${Date.now()}`;
      const newSession: ViewerSession = {
        id: sessionId,
        tunnelId: tunnelInfo.id,
        username,
        password,
        instanceName: instance,
        project,
        zone,
        status: 'connecting',
      };

      setSessions(prev => [...prev, newSession]);
      setActiveSessionId(sessionId);
    } catch (error) {
      console.error('Failed to start session:', error);
      alert('Failed to start session: ' + error);
    }
  };

  // Update session status
  const updateSessionStatus = useCallback((sessionId: string, status: ViewerSession['status'], error?: string) => {
    setSessions(prev =>
      prev.map(s =>
        s.id === sessionId
          ? { ...s, status, error }
          : s
      )
    );
  }, []);

  // Close a session
  const handleCloseSession = useCallback(async (sessionId: string) => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.StopViewerSession(sessionId);
    } catch (err) {
      console.error('Failed to stop viewer session:', err);
    }

    setSessions(prev => prev.filter(s => s.id !== sessionId));

    // Switch to another session if the closed one was active
    if (activeSessionId === sessionId) {
      setSessions(prev => {
        const remaining = prev.filter(s => s.id !== sessionId);
        if (remaining.length > 0) {
          setActiveSessionId(remaining[0].id);
        } else {
          setActiveSessionId(null);
        }
        return prev;
      });
    }
  }, [activeSessionId]);

  // Save connection and/or connect
  const handleSaveConnection = async (connection: Connection, password?: string, saveConnection: boolean = true) => {
    console.log('handleSaveConnection called:', { connection, password: !!password, saveConnection });
    try {
      // Only save connection if requested
      if (saveConnection) {
        console.log('Saving connection...');
        if (connection.id) {
          // @ts-ignore - Wails binding
          await window.go.main.App.UpdateConnection(connection);
          console.log('Connection updated');
        } else {
          // @ts-ignore - Wails binding
          await window.go.main.App.AddConnection(connection);
          console.log('Connection added');
        }
      }
      setModal('none');

      // Connect if password provided
      if (password) {
        await startSession(
          connection.project,
          connection.zone,
          connection.instance,
          connection.username,
          password
        );
      }

      setSelectedInstance(null);
      setEditingConnection(null);
    } catch (error) {
      console.error('Failed to save connection:', error);
    }
  };

  const handleCloseModal = () => {
    setModal('none');
    setSelectedInstance(null);
    setEditingConnection(null);
  };

  return (
    <div className="app">
      <header className="app-header">
        <h1>IAP Client</h1>
        <div className="header-actions">
          <label className="theme-toggle">
            <span>Dark mode</span>
            <input
              type="checkbox"
              checked={theme === 'dark'}
              onChange={toggleTheme}
            />
            <span className="toggle-slider"></span>
          </label>
          <AuthStatusComponent onAuthChange={setAuthenticated} />
        </div>
      </header>

      {rdpInstalled === false && (
        <div className="warning-banner">
          Microsoft Remote Desktop is not installed.
          <a href="https://apps.apple.com/app/microsoft-remote-desktop/id1295203466" target="_blank" rel="noopener noreferrer">
            Install from App Store
          </a>
        </div>
      )}

      {authenticated ? (
        <div className="app-container">
          {/* Left Sidebar */}
          <Sidebar
            sessions={sessions}
            showWindowsOnly={showWindowsOnly}
            onToggleWindowsOnly={setShowWindowsOnly}
            onConnectInstance={handleInstanceSelect}
            onConnectSaved={handleConnectSaved}
            onEditConnection={(conn) => {
              setEditingConnection(conn);
              setSelectedInstance(null);
              setModal('connection-form');
            }}
            onDeleteConnection={async (connectionId) => {
              try {
                // @ts-ignore - Wails binding
                await window.go.main.App.DeleteConnection(connectionId);
              } catch (error) {
                console.error('Failed to delete connection:', error);
              }
            }}
          />

          {/* Main Content Area */}
          <div className="main-content">
            {sessions.length > 0 ? (
              <>
                <SessionTabs
                  sessions={sessions}
                  activeSessionId={activeSessionId}
                  onSelectSession={setActiveSessionId}
                  onCloseSession={handleCloseSession}
                />
                <div className="session-content">
                  {sessions.map(session => (
                    <SessionViewer
                      key={session.id}
                      session={session}
                      isActive={session.id === activeSessionId}
                      onStatusChange={(status, error) => updateSessionStatus(session.id, status, error)}
                      onDisconnect={() => handleCloseSession(session.id)}
                      style={{ display: session.id === activeSessionId ? 'flex' : 'none' }}
                    />
                  ))}
                </div>
              </>
            ) : (
              <div className="welcome-panel">
                <div className="welcome-content">
                  <h2>Connect to a VM</h2>
                  <p>Select a virtual machine from the sidebar to start a remote desktop session.</p>
                  <ul>
                    <li>Browse VMs in your GCP projects</li>
                    <li>Open multiple sessions in tabs</li>
                    <li>Switch between sessions instantly</li>
                  </ul>
                </div>
              </div>
            )}
          </div>

          {/* Connection Form Modal */}
          {modal === 'connection-form' && (
            <div className="modal-overlay" onClick={handleCloseModal}>
              <div className="modal-content" onClick={(e) => e.stopPropagation()}>
                <ConnectionForm
                  instance={selectedInstance || undefined}
                  connection={editingConnection || undefined}
                  onSave={handleSaveConnection}
                  onCancel={handleCloseModal}
                />
              </div>
            </div>
          )}
        </div>
      ) : (
        <div className="unauthenticated-view">
          <div className="welcome-message">
            <h2>Welcome to IAP Client for macOS</h2>
            <p>Connect to Google Cloud Windows VMs through Identity-Aware Proxy tunnels.</p>
            <ul>
              <li>Browse your GCP projects and Windows VMs</li>
              <li>Create secure IAP tunnels</li>
              <li>Launch RDP connections with one click</li>
              <li>Store credentials securely in macOS Keychain</li>
            </ul>
            <div className="gcloud-instructions">
              <h3>Getting Started</h3>
              <p>Authenticate using the <strong>gcloud CLI</strong> from your terminal:</p>
              <div className="code-block">
                <code>gcloud auth application-default login \<br/>&nbsp;&nbsp;--scopes=https://www.googleapis.com/auth/cloud-platform,https://www.googleapis.com/auth/compute</code>
              </div>
              <p className="help-text">
                Don't have the gcloud CLI?{' '}
                <a href="https://cloud.google.com/sdk/docs/install" target="_blank" rel="noopener noreferrer">
                  Install the Google Cloud SDK
                </a>
              </p>
              <p className="signin-prompt">After authenticating, click "Refresh Auth" above to continue.</p>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

export default App;
