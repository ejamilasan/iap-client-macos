import { useState, useEffect } from 'react';
import { Project, Instance, Connection, Tunnel, ViewerSession } from '../types';

interface SidebarProps {
  sessions: ViewerSession[];
  showWindowsOnly: boolean;
  onToggleWindowsOnly: (show: boolean) => void;
  onConnectInstance: (instance: Instance) => void;
  onConnectSaved: (connection: Connection, password?: string) => void;
  onEditConnection: (connection: Connection) => void;
  onDeleteConnection: (connectionId: string) => Promise<void>;
}

export function Sidebar({
  sessions,
  showWindowsOnly,
  onToggleWindowsOnly,
  onConnectInstance,
  onConnectSaved,
  onEditConnection,
  onDeleteConnection
}: SidebarProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [connections, setConnections] = useState<Connection[]>([]);
  const [tunnels, setTunnels] = useState<Tunnel[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [loadingProjects, setLoadingProjects] = useState(false);
  const [loadingInstances, setLoadingInstances] = useState(false);
  const [searchTerm, setSearchTerm] = useState('');
  const [expandedSection, setExpandedSection] = useState<'vms' | 'connections' | 'tunnels'>('vms');

  useEffect(() => {
    loadProjects();
    loadConnections();
    loadTunnels();

    // Poll for tunnels and connections to stay in sync
    const tunnelInterval = setInterval(loadTunnels, 5000);
    const connectionInterval = setInterval(loadConnections, 2000);
    return () => {
      clearInterval(tunnelInterval);
      clearInterval(connectionInterval);
    };
  }, []);

  useEffect(() => {
    if (selectedProject) {
      loadInstances(selectedProject);
    } else {
      setInstances([]);
    }
  }, [selectedProject, showWindowsOnly]);

  const loadProjects = async () => {
    setLoadingProjects(true);
    try {
      // @ts-ignore - Wails binding
      const projectList = await window.go.main.App.ListProjects();
      setProjects(projectList || []);
      if (projectList?.length > 0) {
        setSelectedProject(projectList[0].id);
      }
    } catch (err) {
      console.error('Failed to load projects:', err);
    } finally {
      setLoadingProjects(false);
    }
  };

  const loadInstances = async (projectId: string) => {
    setLoadingInstances(true);
    try {
      // @ts-ignore - Wails binding
      const instanceList = showWindowsOnly
        // @ts-ignore
        ? await window.go.main.App.ListWindowsInstances(projectId)
        // @ts-ignore
        : await window.go.main.App.ListInstances(projectId);
      setInstances(instanceList || []);
    } catch (err) {
      console.error('Failed to load instances:', err);
    } finally {
      setLoadingInstances(false);
    }
  };

  const loadConnections = async () => {
    try {
      // @ts-ignore - Wails binding
      const conns = await window.go.main.App.GetConnections();
      setConnections(conns || []);
    } catch (error) {
      console.error('Failed to load connections:', error);
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

  const handleConnectWithPassword = async (conn: Connection) => {
    try {
      // @ts-ignore - Wails binding
      const password = await window.go.main.App.GetPassword(conn.instance, conn.username);
      onConnectSaved(conn, password || undefined);
    } catch {
      onConnectSaved(conn, undefined);
    }
  };

  const isInstanceConnected = (inst: Instance): boolean => {
    return sessions.some(
      s => s.instanceName === inst.name && s.project === inst.project && s.zone === inst.zone
    );
  };

  const filteredInstances = instances.filter(inst =>
    inst.name.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusClass = (status: string) => {
    switch (status.toUpperCase()) {
      case 'RUNNING': return 'status-running';
      case 'STOPPED': return 'status-stopped';
      default: return 'status-other';
    }
  };

  return (
    <div className="sidebar">
      {/* Header */}
      <div className="sidebar-header">
        <span className="sidebar-title">Resources</span>
        <button onClick={loadProjects} className="sidebar-refresh" title="Refresh">
          &#x21bb;
        </button>
      </div>

      {/* VMs Section */}
      <div className="sidebar-section">
        <div
          className={`section-header ${expandedSection === 'vms' ? 'expanded' : ''}`}
          onClick={() => setExpandedSection(expandedSection === 'vms' ? 'connections' : 'vms')}
        >
          <span className="section-icon">{expandedSection === 'vms' ? '▼' : '▶'}</span>
          <span className="section-title">Virtual Machines</span>
          <span className="section-count">{instances.length}</span>
        </div>

        {expandedSection === 'vms' && (
          <div className="section-content">
            {/* Project selector */}
            <div className="sidebar-project-select">
              <select
                value={selectedProject}
                onChange={(e) => setSelectedProject(e.target.value)}
                disabled={loadingProjects}
              >
                {loadingProjects ? (
                  <option>Loading...</option>
                ) : projects.length === 0 ? (
                  <option>No projects</option>
                ) : (
                  projects.map(p => (
                    <option key={p.id} value={p.id}>{p.id}</option>
                  ))
                )}
              </select>
            </div>

            {/* Filter */}
            <div className="sidebar-filter">
              <input
                type="text"
                placeholder="Filter VMs..."
                value={searchTerm}
                onChange={(e) => setSearchTerm(e.target.value)}
              />
              <label className="sidebar-checkbox">
                <input
                  type="checkbox"
                  checked={showWindowsOnly}
                  onChange={(e) => onToggleWindowsOnly(e.target.checked)}
                />
                Windows only
              </label>
            </div>

            {/* Instance list */}
            <div className="sidebar-list">
              {loadingInstances ? (
                <div className="sidebar-loading">Loading...</div>
              ) : filteredInstances.length === 0 ? (
                <div className="sidebar-empty">No VMs found</div>
              ) : (
                filteredInstances.map(inst => (
                  <div
                    key={`${inst.project}-${inst.zone}-${inst.name}`}
                    className={`sidebar-item ${isInstanceConnected(inst) ? 'connected' : ''}`}
                  >
                    <div className="item-info">
                      <span className={`item-status ${getStatusClass(inst.status)}`}></span>
                      <span className="item-name" title={inst.name}>{inst.name}</span>
                    </div>
                    <button
                      onClick={() => onConnectInstance(inst)}
                      disabled={inst.status !== 'RUNNING' || isInstanceConnected(inst)}
                      className="item-action"
                      title={isInstanceConnected(inst) ? 'Already connected' : 'Connect'}
                    >
                      {isInstanceConnected(inst) ? '...' : '+'}
                    </button>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>

      {/* Saved Connections Section */}
      <div className="sidebar-section">
        <div
          className={`section-header ${expandedSection === 'connections' ? 'expanded' : ''}`}
          onClick={() => setExpandedSection(expandedSection === 'connections' ? 'vms' : 'connections')}
        >
          <span className="section-icon">{expandedSection === 'connections' ? '▼' : '▶'}</span>
          <span className="section-title">Saved Connections</span>
          <span className="section-count">{connections.length}</span>
        </div>

        {expandedSection === 'connections' && (
          <div className="section-content">
            <div className="sidebar-list">
              {connections.length === 0 ? (
                <div className="sidebar-empty">No saved connections</div>
              ) : (
                connections.map(conn => {
                  const isConnected = sessions.some(
                    s => s.instanceName === conn.instance && s.project === conn.project
                  );
                  return (
                    <div
                      key={conn.id}
                      className={`sidebar-item connection-item ${isConnected ? 'connected' : ''}`}
                    >
                      <div className="item-info">
                        <span className={`item-status ${isConnected ? 'status-running' : ''}`}></span>
                        <span className="item-name" title={`${conn.instance} (${conn.project})`}>
                          {conn.name || conn.instance}
                        </span>
                      </div>
                      <div className="item-actions">
                        <button
                          onClick={() => handleConnectWithPassword(conn)}
                          disabled={isConnected}
                          className="item-action connect-btn"
                          title={isConnected ? 'Already connected' : 'Connect'}
                        >
                          {isConnected ? '...' : '+'}
                        </button>
                        <button
                          onClick={() => onEditConnection(conn)}
                          className="item-action edit-btn"
                          title="Edit connection"
                        >
                          &#9998;
                        </button>
                        <button
                          onClick={async (e) => {
                            e.stopPropagation();
                            e.preventDefault();
                            console.log('Delete button clicked for:', conn.id, conn);
                            try {
                              await onDeleteConnection(conn.id);
                              console.log('Delete completed, reloading...');
                              loadConnections();
                            } catch (err) {
                              console.error('Delete failed:', err);
                            }
                          }}
                          className="item-action delete-btn"
                          title="Delete connection"
                        >
                          &times;
                        </button>
                      </div>
                    </div>
                  );
                })
              )}
            </div>
          </div>
        )}
      </div>

      {/* Active Tunnels Section */}
      <div className="sidebar-section">
        <div
          className={`section-header ${expandedSection === 'tunnels' ? 'expanded' : ''}`}
          onClick={() => setExpandedSection(expandedSection === 'tunnels' ? 'vms' : 'tunnels')}
        >
          <span className="section-icon">{expandedSection === 'tunnels' ? '▼' : '▶'}</span>
          <span className="section-title">Active Tunnels</span>
          <span className="section-count">{tunnels.length}</span>
        </div>

        {expandedSection === 'tunnels' && (
          <div className="section-content">
            <div className="sidebar-list">
              {tunnels.length === 0 ? (
                <div className="sidebar-empty">No active tunnels</div>
              ) : (
                tunnels.map(tunnel => (
                  <div key={tunnel.id} className="sidebar-item tunnel-item">
                    <div className="item-info">
                      <span className="item-status status-running"></span>
                      <span className="item-name" title={tunnel.instance}>
                        {tunnel.instance}
                      </span>
                    </div>
                    <div className="item-actions tunnel-actions-inline">
                      <span className="tunnel-port">:{tunnel.localPort}</span>
                      <button
                        onClick={async (e) => {
                          e.stopPropagation();
                          try {
                            // @ts-ignore - Wails binding
                            await window.go.main.App.StopTunnel(tunnel.id);
                            loadTunnels();
                          } catch (err) {
                            console.error('Failed to stop tunnel:', err);
                          }
                        }}
                        className="item-action stop-btn"
                        title="Stop tunnel"
                      >
                        &times;
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
