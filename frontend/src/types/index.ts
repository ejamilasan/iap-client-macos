// Type definitions for the IAP Desktop app

export interface Project {
  id: string;
  name: string;
  number: number;
}

export interface Instance {
  name: string;
  zone: string;
  status: string;
  machineType: string;
  internalIP: string;
  externalIP: string;
  project: string;
  isWindows: boolean;
}

export interface Connection {
  id: string;
  name: string;
  project: string;
  zone: string;
  instance: string;
  username: string;
  domain?: string;
  createdAt: string;
  updatedAt: string;
  lastUsedAt?: string;
  rdpSettings?: RDPSettings;
}

export interface RDPSettings {
  fullScreen: boolean;
  screenWidth: number;
  screenHeight: number;
  colorDepth: number;
  audioMode: number;
  driveRedirect: boolean;
  clipboardShare: boolean;
}

export interface Tunnel {
  id: string;
  project: string;
  zone: string;
  instance: string;
  remotePort: number;
  localPort: number;
  status: string;
  startedAt: string;
  bytesSent: number;
  bytesReceived: number;
  connections: number;
}

export interface AuthStatus {
  authenticated: boolean;
  expiry?: string;
}

export interface LogEntry {
  tunnelId: string;
  timestamp: string;
  level: string;
  message: string;
}

export interface AppSettings {
  defaultProject?: string;
  autoConnect: boolean;
  showWindowsOnly: boolean;
  rememberPasswords: boolean;
  preferEmbeddedViewer?: boolean;
  viewerQuality?: number;
}

export interface ViewerState {
  tunnelId: string;
  username: string;
  password: string;
  instanceName: string;
}

export interface ViewerSession {
  id: string;
  tunnelId: string;
  username: string;
  password: string;
  instanceName: string;
  project: string;
  zone: string;
  status: 'connecting' | 'connected' | 'error' | 'disconnected';
  error?: string;
}
