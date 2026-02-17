import { useEffect, useRef, useState, useCallback, CSSProperties } from 'react';
import { EventsOn } from '../../wailsjs/runtime/runtime';
import { ViewerSession } from '../types';

// Map JavaScript key codes to RDP scan codes
const keyCodeToScanCode: { [key: number]: number } = {
  8: 0x0E,    // Backspace
  9: 0x0F,    // Tab
  13: 0x1C,   // Enter
  16: 0x2A,   // Shift (left)
  17: 0x1D,   // Control (left)
  18: 0x38,   // Alt (left)
  19: 0x45,   // Pause
  20: 0x3A,   // Caps Lock
  27: 0x01,   // Escape
  32: 0x39,   // Space
  33: 0x49,   // Page Up
  34: 0x51,   // Page Down
  35: 0x4F,   // End
  36: 0x47,   // Home
  37: 0x4B,   // Left Arrow
  38: 0x48,   // Up Arrow
  39: 0x4D,   // Right Arrow
  40: 0x50,   // Down Arrow
  45: 0x52,   // Insert
  46: 0x53,   // Delete
  48: 0x0B,   // 0
  49: 0x02,   // 1
  50: 0x03,   // 2
  51: 0x04,   // 3
  52: 0x05,   // 4
  53: 0x06,   // 5
  54: 0x07,   // 6
  55: 0x08,   // 7
  56: 0x09,   // 8
  57: 0x0A,   // 9
  65: 0x1E,   // A
  66: 0x30,   // B
  67: 0x2E,   // C
  68: 0x20,   // D
  69: 0x12,   // E
  70: 0x21,   // F
  71: 0x22,   // G
  72: 0x23,   // H
  73: 0x17,   // I
  74: 0x24,   // J
  75: 0x25,   // K
  76: 0x26,   // L
  77: 0x32,   // M
  78: 0x31,   // N
  79: 0x18,   // O
  80: 0x19,   // P
  81: 0x10,   // Q
  82: 0x13,   // R
  83: 0x1F,   // S
  84: 0x14,   // T
  85: 0x16,   // U
  86: 0x2F,   // V
  87: 0x11,   // W
  88: 0x2D,   // X
  89: 0x15,   // Y
  90: 0x2C,   // Z
  91: 0x5B,   // Left Windows
  92: 0x5C,   // Right Windows
  112: 0x3B,  // F1
  113: 0x3C,  // F2
  114: 0x3D,  // F3
  115: 0x3E,  // F4
  116: 0x3F,  // F5
  117: 0x40,  // F6
  118: 0x41,  // F7
  119: 0x42,  // F8
  120: 0x43,  // F9
  121: 0x44,  // F10
  122: 0x57,  // F11
  123: 0x58,  // F12
  186: 0x27,  // ;
  187: 0x0D,  // =
  188: 0x33,  // ,
  189: 0x0C,  // -
  190: 0x34,  // .
  191: 0x35,  // /
  192: 0x29,  // `
  219: 0x1A,  // [
  220: 0x2B,  // \
  221: 0x1B,  // ]
  222: 0x28,  // '
};

interface SessionViewerProps {
  session: ViewerSession;
  isActive: boolean;
  onStatusChange: (status: ViewerSession['status'], error?: string) => void;
  onDisconnect: () => void;
  style?: CSSProperties;
}

export function SessionViewer({
  session,
  isActive,
  onStatusChange,
  onDisconnect,
  style
}: SessionViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const latestFrameRef = useRef<string | null>(null);
  const [scale, setScale] = useState(1);
  const [resolution, setResolution] = useState({ width: 1280, height: 800 });

  // Render a frame to the canvas
  const renderFrame = useCallback((base64Data: string) => {
    const canvas = canvasRef.current;
    if (!canvas) return false;

    const ctx = canvas.getContext('2d');
    if (!ctx) return false;

    const img = new Image();
    img.onload = () => {
      ctx.drawImage(img, 0, 0);
    };
    img.src = 'data:image/jpeg;base64,' + base64Data;
    return true;
  }, []);

  // Handle frame updates
  const handleFrame = useCallback((base64Data: string) => {
    latestFrameRef.current = base64Data;
    renderFrame(base64Data);
  }, [renderFrame]);

  // When session becomes connected, render latest frame
  useEffect(() => {
    if (session.status === 'connected' && latestFrameRef.current) {
      renderFrame(latestFrameRef.current);
    }
  }, [session.status, renderFrame]);

  // Track if session has been started to prevent duplicate starts
  const sessionStartedRef = useRef(false);

  // Subscribe to session-specific events
  useEffect(() => {
    let mounted = true;
    const sessionId = session.id;

    // Subscribe to session-specific events
    const unsubFrame = EventsOn(`rdp-frame-${sessionId}`, (base64Data: string) => {
      if (!mounted) return;
      handleFrame(base64Data);
    });

    const unsubConnected = EventsOn(`rdp-connected-${sessionId}`, () => {
      if (!mounted) return;
      onStatusChange('connected');
    });

    const unsubError = EventsOn(`rdp-error-${sessionId}`, (err: string) => {
      if (!mounted) return;
      onStatusChange('error', err);
    });

    const unsubDisconnected = EventsOn(`rdp-disconnected-${sessionId}`, () => {
      if (!mounted) return;
      onStatusChange('disconnected');
      onDisconnect();
    });

    // Listen for resolution changes from the server
    const unsubResolution = EventsOn(`rdp-resolution-${sessionId}`, (data: { width: number; height: number }) => {
      if (!mounted) return;
      console.log(`[SessionViewer] Resolution changed to ${data.width}x${data.height}`);
      setResolution({ width: data.width, height: data.height });
    });

    // Start the viewer session only if not already started
    const startSession = async () => {
      if (!mounted) return;
      if (sessionStartedRef.current) {
        console.log(`[SessionViewer] Session ${sessionId} already started, skipping`);
        return;
      }
      sessionStartedRef.current = true;

      // Detect container size for resolution
      let width = 1280;
      let height = 800;
      if (containerRef.current) {
        const rect = containerRef.current.getBoundingClientRect();
        // Use container size, but ensure minimum dimensions
        width = Math.max(800, Math.floor(rect.width));
        height = Math.max(600, Math.floor(rect.height));
        // Round to nearest 8 pixels for better RDP compatibility
        width = Math.floor(width / 8) * 8;
        height = Math.floor(height / 8) * 8;
      }
      setResolution({ width, height });
      console.log(`[SessionViewer] Starting session with resolution ${width}x${height}`);

      try {
        // @ts-ignore - Wails binding
        await window.go.main.App.StartViewerSession(
          sessionId,
          session.tunnelId,
          session.username,
          session.password,
          width,
          height
        );
      } catch (err: any) {
        if (!mounted) return;
        onStatusChange('error', err.toString());
      }
    };
    startSession();

    return () => {
      mounted = false;
      if (unsubFrame) unsubFrame();
      if (unsubConnected) unsubConnected();
      if (unsubError) unsubError();
      if (unsubDisconnected) unsubDisconnected();
      if (unsubResolution) unsubResolution();
      // Don't stop the session here - let the parent manage lifecycle
      // The parent will call StopViewerSession when closing the session
    };
  }, [session.id, session.tunnelId, session.username, session.password, handleFrame, onStatusChange, onDisconnect]);

  // Calculate canvas display size to fill container while maintaining aspect ratio
  const [canvasStyle, setCanvasStyle] = useState<CSSProperties>({});

  useEffect(() => {
    const updateCanvasSize = () => {
      if (!containerRef.current) return;
      const containerWidth = containerRef.current.clientWidth;
      const containerHeight = containerRef.current.clientHeight;

      // Calculate the display size that fits the container while maintaining aspect ratio
      const aspectRatio = resolution.width / resolution.height;
      let displayWidth = containerWidth;
      let displayHeight = containerWidth / aspectRatio;

      if (displayHeight > containerHeight) {
        displayHeight = containerHeight;
        displayWidth = containerHeight * aspectRatio;
      }

      setCanvasStyle({
        width: `${displayWidth}px`,
        height: `${displayHeight}px`,
      });

      // Also calculate scale for mouse coordinate translation
      setScale(displayWidth / resolution.width);
    };

    updateCanvasSize();
    window.addEventListener('resize', updateCanvasSize);

    const resizeObserver = new ResizeObserver(updateCanvasSize);
    if (containerRef.current) {
      resizeObserver.observe(containerRef.current);
    }

    return () => {
      window.removeEventListener('resize', updateCanvasSize);
      resizeObserver.disconnect();
    };
  }, [resolution.width, resolution.height]);

  const getCanvasCoords = (e: React.MouseEvent): { x: number; y: number } => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };

    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) / scale;
    const y = (e.clientY - rect.top) / scale;
    return { x: Math.round(x), y: Math.round(y) };
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!isActive || session.status !== 'connected') return;
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionMouseMove(session.id, x, y);
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    if (!isActive || session.status !== 'connected') return;
    e.preventDefault();
    canvasRef.current?.focus();
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionMouseClick(session.id, x, y, e.button, true);
  };

  const handleMouseUp = (e: React.MouseEvent) => {
    if (!isActive || session.status !== 'connected') return;
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionMouseClick(session.id, x, y, e.button, false);
  };

  const handleWheel = (e: React.WheelEvent) => {
    if (!isActive || session.status !== 'connected') return;
    e.preventDefault();
    const scroll = e.deltaY > 0 ? -120 : 120;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionMouseWheel(session.id, scroll);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!isActive || session.status !== 'connected') return;
    e.preventDefault();
    e.stopPropagation();

    const scanCode = keyCodeToScanCode[e.keyCode] || e.keyCode;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionKeyboard(session.id, scanCode, true);
  };

  const handleKeyUp = (e: React.KeyboardEvent) => {
    if (!isActive || session.status !== 'connected') return;
    e.preventDefault();
    e.stopPropagation();

    const scanCode = keyCodeToScanCode[e.keyCode] || e.keyCode;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerSessionKeyboard(session.id, scanCode, false);
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
  };

  return (
    <div className="session-viewer" ref={containerRef} style={style}>
      {session.status === 'error' && (
        <div className="session-error">
          <div className="error-content">
            <span className="error-icon">!</span>
            <span className="error-text">{session.error || 'Connection failed'}</span>
          </div>
          <button onClick={onDisconnect} className="btn btn-secondary btn-sm">
            Close
          </button>
        </div>
      )}

      {session.status === 'connecting' && (
        <div className="session-connecting">
          <span className="spinner"></span>
          <span>Connecting to {session.instanceName}...</span>
        </div>
      )}

      {(session.status === 'connected' || session.status === 'connecting') && (
        <div className="session-canvas-container">
          <canvas
            ref={canvasRef}
            width={resolution.width}
            height={resolution.height}
            className="session-canvas"
            tabIndex={isActive ? 0 : -1}
            style={canvasStyle}
            onMouseMove={handleMouseMove}
            onMouseDown={handleMouseDown}
            onMouseUp={handleMouseUp}
            onWheel={handleWheel}
            onKeyDown={handleKeyDown}
            onKeyUp={handleKeyUp}
            onContextMenu={handleContextMenu}
          />
        </div>
      )}
    </div>
  );
}
