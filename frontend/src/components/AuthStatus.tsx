import { useState, useEffect, useRef } from 'react';

interface AuthStatusType {
  authenticated: boolean;
  gcloudInstalled?: boolean;
  authMethod?: string;
  account?: string;
}

interface AuthStatusProps {
  onAuthChange: (authenticated: boolean) => void;
}

export function AuthStatusComponent({ onAuthChange }: AuthStatusProps) {
  const [authStatus, setAuthStatus] = useState<AuthStatusType | null>(null);
  const [loading, setLoading] = useState(true);
  const [checking, setChecking] = useState(false);
  const [dropdownOpen, setDropdownOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Close dropdown when clicking outside
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(event.target as Node)) {
        setDropdownOpen(false);
      }
    };

    if (dropdownOpen) {
      document.addEventListener('mousedown', handleClickOutside);
    }

    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [dropdownOpen]);

  useEffect(() => {
    checkAuth();
  }, []);

  const checkAuth = async () => {
    setChecking(true);
    try {
      // @ts-ignore - Wails binding
      const status = await window.go.main.App.GetAuthStatus();
      setAuthStatus(status);
      onAuthChange(status.authenticated);
    } catch (error) {
      console.error('Failed to check auth status:', error);
      setAuthStatus({ authenticated: false });
      onAuthChange(false);
    } finally {
      setLoading(false);
      setChecking(false);
    }
  };

  const handleLogout = async () => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.Logout();
      setAuthStatus({ authenticated: false });
      onAuthChange(false);
    } catch (error) {
      console.error('Logout failed:', error);
    }
  };

  if (loading) {
    return <div className="auth-status loading">Checking authentication...</div>;
  }

  return (
    <div className="auth-status">
      {authStatus?.authenticated ? (
        <div className="auth-authenticated" ref={dropdownRef}>
          <button
            className="auth-indicator-btn authenticated"
            onClick={() => setDropdownOpen(!dropdownOpen)}
            title={authStatus.account || 'Signed in'}
          >
            <span className="auth-method">gcloud</span>
            <span className="auth-dropdown-arrow">{dropdownOpen ? '▲' : '▼'}</span>
          </button>
          {dropdownOpen && (
            <div className="auth-dropdown">
              <div className="auth-dropdown-header">
                <div className="auth-dropdown-label">Signed in as</div>
                <div className="auth-dropdown-email">{authStatus.account}</div>
              </div>
              <div className="auth-dropdown-divider" />
              <button
                onClick={() => {
                  handleLogout();
                  setDropdownOpen(false);
                }}
                className="auth-dropdown-item"
              >
                Sign Out
              </button>
            </div>
          )}
        </div>
      ) : (
        <div className="auth-unauthenticated">
          <button
            onClick={checkAuth}
            disabled={checking}
            className="btn btn-secondary btn-sm"
            title="Re-check gcloud authentication"
          >
            {checking ? 'Checking...' : 'Refresh Auth'}
          </button>
        </div>
      )}
    </div>
  );
}
