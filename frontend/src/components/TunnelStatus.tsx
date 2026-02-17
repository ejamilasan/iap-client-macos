import { useState, useEffect, useRef } from 'react';
import { Tunnel, LogEntry } from '../types';

interface TunnelStatusProps {
  onClose?: () => void;
}

export function TunnelStatus({ onClose }: TunnelStatusProps) {
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [selectedTunnel, setSelectedTunnel] = useState<string | null>(null);
  const logsEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadTunnels();
    const interval = setInterval(loadTunnels, 2000);
    return () => clearInterval(interval);
  }, []);

  useEffect(() => {
    logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [logs]);

  const loadTunnels = async () => {
    try {
      // @ts-ignore - Wails binding
      const tunnelList = await window.go.main.App.ListTunnels();
      setTunnels(tunnelList || []);
    } catch (error) {
      console.error('Failed to load tunnels:', error);
    }
  };

  const handleStopTunnel = async (tunnelId: string) => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.StopTunnel(tunnelId);
      await loadTunnels();
    } catch (error) {
      console.error('Failed to stop tunnel:', error);
    }
  };

  const handleStopAll = async () => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.StopAllTunnels();
      await loadTunnels();
    } catch (error) {
      console.error('Failed to stop tunnels:', error);
    }
  };

  const formatBytes = (bytes: number): string => {
    if (bytes === 0) return '0 B';
    const k = 1024;
    const sizes = ['B', 'KB', 'MB', 'GB'];
    const i = Math.floor(Math.log(bytes) / Math.log(k));
    return parseFloat((bytes / Math.pow(k, i)).toFixed(1)) + ' ' + sizes[i];
  };

  const formatDuration = (startedAt: string): string => {
    const start = new Date(startedAt);
    const now = new Date();
    const diff = Math.floor((now.getTime() - start.getTime()) / 1000);

    if (diff < 60) return `${diff}s`;
    if (diff < 3600) return `${Math.floor(diff / 60)}m ${diff % 60}s`;
    return `${Math.floor(diff / 3600)}h ${Math.floor((diff % 3600) / 60)}m`;
  };

  if (tunnels.length === 0) {
    return (
      <div className="tunnel-status empty">
        <div className="empty-state">
          <h3>No Active Tunnels</h3>
          <p>Connect to an instance to start a tunnel.</p>
        </div>
      </div>
    );
  }

  return (
    <div className="tunnel-status">
      <div className="tunnel-header">
        <h2>Active Tunnels ({tunnels.length})</h2>
        {tunnels.length > 1 && (
          <button onClick={handleStopAll} className="btn btn-warning btn-sm">
            Stop All
          </button>
        )}
      </div>

      <div className="tunnel-list">
        {tunnels.map(tunnel => (
          <div
            key={tunnel.id}
            className={`tunnel-card ${selectedTunnel === tunnel.id ? 'selected' : ''}`}
            onClick={() => setSelectedTunnel(tunnel.id === selectedTunnel ? null : tunnel.id)}
          >
            <div className="tunnel-main">
              <div className="tunnel-info">
                <span className="tunnel-instance">{tunnel.instance}</span>
                <span className="tunnel-project">{tunnel.project}</span>
              </div>
              <div className="tunnel-port">
                <span className="port-label">localhost:</span>
                <span className="port-number">{tunnel.localPort}</span>
                <span className="port-arrow">-&gt;</span>
                <span className="port-number">{tunnel.remotePort}</span>
              </div>
            </div>

            <div className="tunnel-stats">
              <div className="stat">
                <span className="stat-label">Status</span>
                <span className={`stat-value status-${tunnel.status}`}>{tunnel.status}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Duration</span>
                <span className="stat-value">{formatDuration(tunnel.startedAt)}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Connections</span>
                <span className="stat-value">{tunnel.connections}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Sent</span>
                <span className="stat-value">{formatBytes(tunnel.bytesSent)}</span>
              </div>
              <div className="stat">
                <span className="stat-label">Received</span>
                <span className="stat-value">{formatBytes(tunnel.bytesReceived)}</span>
              </div>
            </div>

            <div className="tunnel-actions">
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleStopTunnel(tunnel.id);
                }}
                className="btn btn-warning btn-sm"
              >
                Stop
              </button>
            </div>
          </div>
        ))}
      </div>

      {selectedTunnel && (
        <div className="tunnel-logs">
          <h3>Logs</h3>
          <div className="log-entries">
            {logs
              .filter(l => l.tunnelId === selectedTunnel)
              .map((log, i) => (
                <div key={i} className={`log-entry log-${log.level}`}>
                  <span className="log-time">
                    {new Date(log.timestamp).toLocaleTimeString()}
                  </span>
                  <span className="log-level">[{log.level}]</span>
                  <span className="log-message">{log.message}</span>
                </div>
              ))
            }
            <div ref={logsEndRef} />
          </div>
        </div>
      )}
    </div>
  );
}
