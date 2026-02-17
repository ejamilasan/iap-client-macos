import { useState, useEffect } from 'react';
import { Connection, Tunnel } from '../types';

interface ConnectionListProps {
  onConnect: (connection: Connection, password?: string) => void;
  onEdit: (connection: Connection) => void;
  onDelete: (connectionId: string) => void;
}

export function ConnectionList({ onConnect, onEdit, onDelete }: ConnectionListProps) {
  const [connections, setConnections] = useState<Connection[]>([]);
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    loadConnections();
    loadTunnels();

    // Poll for tunnel updates
    const interval = setInterval(loadTunnels, 5000);
    return () => clearInterval(interval);
  }, []);

  const loadConnections = async () => {
    try {
      // @ts-ignore - Wails binding
      const conns = await window.go.main.App.GetConnections();
      setConnections(conns || []);
    } catch (error) {
      console.error('Failed to load connections:', error);
    } finally {
      setLoading(false);
    }
  };

  const loadTunnels = async () => {
    try {
      // @ts-ignore - Wails binding
      const tunnelList = await window.go.main.App.ListTunnels();
      setTunnels(tunnelList || []);
    } catch (error) {
      console.error('Failed to load tunnels:', error);
    }
  };

  const getTunnelForConnection = (conn: Connection): Tunnel | undefined => {
    return tunnels.find(t =>
      t.project === conn.project &&
      t.zone === conn.zone &&
      t.instance === conn.instance
    );
  };

  const handleDisconnect = async (tunnelId: string) => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.StopTunnel(tunnelId);
      await loadTunnels();
    } catch (error) {
      console.error('Failed to stop tunnel:', error);
    }
  };

  const handleDelete = async (connectionId: string) => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.DeleteConnection(connectionId);
      // Reload connections after delete
      await loadConnections();
    } catch (error) {
      console.error('Failed to delete connection:', error);
      alert('Failed to delete connection: ' + error);
    }
  };

  const handleConnectWithPassword = async (conn: Connection) => {
    try {
      // Try to get password from keychain
      // @ts-ignore - Wails binding
      const password = await window.go.main.App.GetPassword(conn.instance, conn.username);
      onConnect(conn, password || undefined);
    } catch (error) {
      // No password in keychain, connect without password (external RDP will prompt)
      onConnect(conn, undefined);
    }
  };

  const formatDate = (dateStr: string | undefined) => {
    if (!dateStr) return 'Never';
    return new Date(dateStr).toLocaleDateString();
  };

  if (loading) {
    return <div className="loading">Loading connections...</div>;
  }

  if (connections.length === 0) {
    return (
      <div className="connection-list empty">
        <div className="empty-state">
          <h3>No Saved Connections</h3>
          <p>Connect to an instance from the browser to create a saved connection.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="connection-list">
      <h2>Saved Connections</h2>
      <div className="connections">
        {connections.map(conn => {
          const tunnel = getTunnelForConnection(conn);
          const isConnected = tunnel?.status === 'running';

          return (
            <div key={conn.id} className={`connection-card ${isConnected ? 'connected' : ''}`}>
              <div className="connection-header">
                <h3>{conn.name || conn.instance}</h3>
                {isConnected && (
                  <span className="connected-badge">Connected</span>
                )}
              </div>

              <div className="connection-details">
                <div className="detail">
                  <span className="label">Project:</span>
                  <span className="value">{conn.project}</span>
                </div>
                <div className="detail">
                  <span className="label">Instance:</span>
                  <span className="value">{conn.instance}</span>
                </div>
                <div className="detail">
                  <span className="label">Zone:</span>
                  <span className="value">{conn.zone}</span>
                </div>
                <div className="detail">
                  <span className="label">Username:</span>
                  <span className="value">{conn.username || 'Not set'}</span>
                </div>
                <div className="detail">
                  <span className="label">Last Used:</span>
                  <span className="value">{formatDate(conn.lastUsedAt)}</span>
                </div>
              </div>

              {isConnected && tunnel && (
                <div className="tunnel-info">
                  <div className="tunnel-stat">
                    <span className="label">Local Port:</span>
                    <span className="value">{tunnel.localPort}</span>
                  </div>
                  <div className="tunnel-stat">
                    <span className="label">Connections:</span>
                    <span className="value">{tunnel.connections}</span>
                  </div>
                </div>
              )}

              <div className="connection-actions">
                {isConnected ? (
                  <>
                    <button
                      onClick={() => handleDisconnect(tunnel!.id)}
                      className="btn btn-warning btn-sm"
                    >
                      Disconnect
                    </button>
                    <button
                      onClick={() => handleConnectWithPassword(conn)}
                      className="btn btn-primary btn-sm"
                    >
                      Launch RDP
                    </button>
                  </>
                ) : (
                  <button
                    onClick={() => handleConnectWithPassword(conn)}
                    className="btn btn-primary btn-sm"
                  >
                    Connect
                  </button>
                )}
                <button
                  onClick={() => onEdit(conn)}
                  className="btn btn-secondary btn-sm"
                >
                  Edit
                </button>
                <button
                  onClick={() => handleDelete(conn.id)}
                  className="btn btn-danger btn-sm"
                >
                  Delete
                </button>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
}
