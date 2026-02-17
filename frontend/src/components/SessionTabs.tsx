import { ViewerSession } from '../types';

interface SessionTabsProps {
  sessions: ViewerSession[];
  activeSessionId: string | null;
  onSelectSession: (sessionId: string) => void;
  onCloseSession: (sessionId: string) => void;
}

export function SessionTabs({
  sessions,
  activeSessionId,
  onSelectSession,
  onCloseSession
}: SessionTabsProps) {
  if (sessions.length === 0) {
    return null;
  }

  return (
    <div className="session-tabs">
      {sessions.map((session) => (
        <div
          key={session.id}
          className={`session-tab ${session.id === activeSessionId ? 'active' : ''}`}
          onClick={() => onSelectSession(session.id)}
        >
          <span className={`tab-status tab-status-${session.status}`}></span>
          <span className="tab-name">{session.instanceName}</span>
          <button
            className="tab-close"
            onClick={(e) => {
              e.stopPropagation();
              onCloseSession(session.id);
            }}
            title="Close session"
          >
            &times;
          </button>
        </div>
      ))}
    </div>
  );
}
