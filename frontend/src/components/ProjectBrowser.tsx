import { useState, useEffect } from 'react';
import { Project, Instance } from '../types';

interface ProjectBrowserProps {
  onInstanceSelect: (instance: Instance) => void;
  showWindowsOnly: boolean;
}

export function ProjectBrowser({ onInstanceSelect, showWindowsOnly }: ProjectBrowserProps) {
  const [projects, setProjects] = useState<Project[]>([]);
  const [instances, setInstances] = useState<Instance[]>([]);
  const [selectedProject, setSelectedProject] = useState<string>('');
  const [loadingProjects, setLoadingProjects] = useState(false);
  const [loadingInstances, setLoadingInstances] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [searchTerm, setSearchTerm] = useState('');

  useEffect(() => {
    loadProjects();
  }, []);

  useEffect(() => {
    if (selectedProject) {
      loadInstances(selectedProject);
    } else {
      setInstances([]);
    }
  }, [selectedProject]);

  const loadProjects = async () => {
    setLoadingProjects(true);
    setError(null);
    try {
      // @ts-ignore - Wails binding
      const projectList = await window.go.main.App.ListProjects();
      setProjects(projectList || []);
      if (projectList?.length > 0) {
        setSelectedProject(projectList[0].id);
      }
    } catch (err) {
      setError('Failed to load projects. Please check your permissions.');
      console.error('Failed to load projects:', err);
    } finally {
      setLoadingProjects(false);
    }
  };

  const loadInstances = async (projectId: string) => {
    setLoadingInstances(true);
    setError(null);
    try {
      // @ts-ignore - Wails binding
      const instanceList = showWindowsOnly
        // @ts-ignore
        ? await window.go.main.App.ListWindowsInstances(projectId)
        // @ts-ignore
        : await window.go.main.App.ListInstances(projectId);
      setInstances(instanceList || []);
    } catch (err) {
      setError('Failed to load instances.');
      console.error('Failed to load instances:', err);
    } finally {
      setLoadingInstances(false);
    }
  };

  const filteredInstances = instances.filter(inst =>
    inst.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
    inst.zone.toLowerCase().includes(searchTerm.toLowerCase())
  );

  const getStatusClass = (status: string) => {
    switch (status.toUpperCase()) {
      case 'RUNNING':
        return 'status-running';
      case 'STOPPED':
        return 'status-stopped';
      case 'TERMINATED':
        return 'status-terminated';
      default:
        return 'status-other';
    }
  };

  return (
    <div className="project-browser">
      <div className="browser-header">
        <h2>GCP Resources</h2>
        <button onClick={loadProjects} className="btn btn-icon" title="Refresh">
          Refresh
        </button>
      </div>

      {error && (
        <div className="error-message">
          {error}
        </div>
      )}

      <div className="project-selector">
        <label htmlFor="project-select">Project:</label>
        <select
          id="project-select"
          value={selectedProject}
          onChange={(e) => setSelectedProject(e.target.value)}
          disabled={loadingProjects}
        >
          {loadingProjects ? (
            <option>Loading projects...</option>
          ) : projects.length === 0 ? (
            <option>No projects found</option>
          ) : (
            projects.map(p => (
              <option key={p.id} value={p.id}>{p.name} ({p.id})</option>
            ))
          )}
        </select>
      </div>

      <div className="instance-search">
        <input
          type="text"
          placeholder="Filter instances..."
          value={searchTerm}
          onChange={(e) => setSearchTerm(e.target.value)}
        />
      </div>

      <div className="instance-list">
        {loadingInstances ? (
          <div className="loading">Loading instances...</div>
        ) : filteredInstances.length === 0 ? (
          <div className="empty">
            {showWindowsOnly ? 'No Windows instances found' : 'No instances found'}
          </div>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Name</th>
                <th>Zone</th>
                <th>Status</th>
                <th>Type</th>
                <th>Actions</th>
              </tr>
            </thead>
            <tbody>
              {filteredInstances.map(inst => (
                <tr key={`${inst.project}-${inst.zone}-${inst.name}`}>
                  <td>
                    <span className="instance-name">
                      {inst.isWindows && <span className="windows-icon" title="Windows">W</span>}
                      {inst.name}
                    </span>
                  </td>
                  <td>{inst.zone}</td>
                  <td>
                    <span className={`status-badge ${getStatusClass(inst.status)}`}>
                      {inst.status}
                    </span>
                  </td>
                  <td>{inst.machineType}</td>
                  <td>
                    <button
                      onClick={() => onInstanceSelect(inst)}
                      disabled={inst.status !== 'RUNNING'}
                      className="btn btn-primary btn-sm"
                    >
                      Connect
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}
