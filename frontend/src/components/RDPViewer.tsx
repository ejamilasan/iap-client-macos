import { useEffect, useRef, useState, useCallback } from 'react';
import { EventsOn, EventsOff } from '../../wailsjs/runtime/runtime';

interface RDPViewerProps {
  tunnelId: string;
  username: string;
  password: string;
  onDisconnect: () => void;
}

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

export function RDPViewer({ tunnelId, username, password, onDisconnect }: RDPViewerProps) {
  const canvasRef = useRef<HTMLCanvasElement>(null);
  const containerRef = useRef<HTMLDivElement>(null);
  const latestFrameRef = useRef<string | null>(null);
  const [connected, setConnected] = useState(false);
  const [connecting, setConnecting] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [scale, setScale] = useState(1);

  // Render a frame to the canvas
  const renderFrame = useCallback((base64Data: string) => {
    const canvas = canvasRef.current;
    if (!canvas) {
      console.log('[RDPViewer] renderFrame: canvas not available');
      return false;
    }

    const ctx = canvas.getContext('2d');
    if (!ctx) {
      console.log('[RDPViewer] renderFrame: context not available');
      return false;
    }

    const img = new Image();
    img.onload = () => {
      console.log('[RDPViewer] renderFrame: drawing image', img.width, 'x', img.height);
      ctx.drawImage(img, 0, 0);
    };
    img.onerror = (e) => {
      console.error('[RDPViewer] renderFrame: image load error', e);
    };
    img.src = 'data:image/jpeg;base64,' + base64Data;
    return true;
  }, []);

  // Handle frame updates - store and try to render
  const handleFrame = useCallback((base64Data: string) => {
    latestFrameRef.current = base64Data;
    renderFrame(base64Data);
  }, [renderFrame]);

  // When canvas becomes available (connected state changes), render latest frame
  useEffect(() => {
    if (connected && latestFrameRef.current) {
      console.log('[RDPViewer] Canvas available, rendering latest frame');
      renderFrame(latestFrameRef.current);
    }
  }, [connected, renderFrame]);

  useEffect(() => {
    let mounted = true;
    console.log('[RDPViewer] Setting up event listeners...');

    // Subscribe to Wails events using proper imports
    const unsubFrame = EventsOn('rdp-frame', (base64Data: string) => {
      if (!mounted) return;
      console.log('[RDPViewer] Received frame, size:', base64Data?.length || 0);
      handleFrame(base64Data);
    });

    const unsubConnected = EventsOn('rdp-connected', () => {
      if (!mounted) return;
      console.log('[RDPViewer] Connected event received');
      setConnected(true);
      setConnecting(false);
      setError(null); // Clear any transient errors when connection succeeds
    });

    const unsubError = EventsOn('rdp-error', (err: string) => {
      if (!mounted) return;
      console.log('[RDPViewer] Error event received:', err);
      setError(err);
      setConnecting(false);
      setConnected(false); // Clear connected state on error
    });

    const unsubDisconnected = EventsOn('rdp-disconnected', () => {
      if (!mounted) return;
      console.log('[RDPViewer] Disconnected event received');
      setConnected(false);
      onDisconnect();
    });

    // Start viewer immediately
    const startViewer = async () => {
      if (!mounted) return;
      try {
        console.log('[RDPViewer] Starting viewer...');
        // @ts-ignore - Wails binding
        await window.go.main.App.StartViewer(tunnelId, username, password);
        console.log('[RDPViewer] StartViewer returned');
      } catch (err: any) {
        if (!mounted) return;
        console.error('[RDPViewer] StartViewer error:', err);
        setError(err.toString());
        setConnecting(false);
      }
    };
    startViewer();

    return () => {
      mounted = false;

      // Unsubscribe from events
      if (unsubFrame) unsubFrame();
      if (unsubConnected) unsubConnected();
      if (unsubError) unsubError();
      if (unsubDisconnected) unsubDisconnected();

      // Stop viewer
      // @ts-ignore - Wails binding
      window.go.main.App.StopViewer?.();
    };
  }, [tunnelId, username, password, handleFrame, onDisconnect]);

  // Calculate scale when container size changes
  useEffect(() => {
    const updateScale = () => {
      if (!containerRef.current || !canvasRef.current) return;
      const containerWidth = containerRef.current.clientWidth;
      const containerHeight = containerRef.current.clientHeight - 50; // Account for toolbar
      const canvasWidth = canvasRef.current.width;
      const canvasHeight = canvasRef.current.height;

      const scaleX = containerWidth / canvasWidth;
      const scaleY = containerHeight / canvasHeight;
      setScale(Math.min(scaleX, scaleY, 1)); // Don't scale up, only down
    };

    updateScale();
    window.addEventListener('resize', updateScale);
    return () => window.removeEventListener('resize', updateScale);
  }, []);

  const getCanvasCoords = (e: React.MouseEvent): { x: number; y: number } => {
    const canvas = canvasRef.current;
    if (!canvas) return { x: 0, y: 0 };

    const rect = canvas.getBoundingClientRect();
    const x = (e.clientX - rect.left) / scale;
    const y = (e.clientY - rect.top) / scale;
    return { x: Math.round(x), y: Math.round(y) };
  };

  const handleMouseMove = (e: React.MouseEvent) => {
    if (!connected) return;
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerMouseMove(x, y);
  };

  const handleMouseDown = (e: React.MouseEvent) => {
    if (!connected) return;
    e.preventDefault();
    canvasRef.current?.focus();
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerMouseClick(x, y, e.button, true);
  };

  const handleMouseUp = (e: React.MouseEvent) => {
    if (!connected) return;
    const { x, y } = getCanvasCoords(e);
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerMouseClick(x, y, e.button, false);
  };

  const handleWheel = (e: React.WheelEvent) => {
    if (!connected) return;
    e.preventDefault();
    const scroll = e.deltaY > 0 ? -120 : 120;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerMouseWheel(scroll);
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!connected) return;
    e.preventDefault();
    e.stopPropagation();

    const scanCode = keyCodeToScanCode[e.keyCode] || e.keyCode;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerKeyboard(scanCode, true);
  };

  const handleKeyUp = (e: React.KeyboardEvent) => {
    if (!connected) return;
    e.preventDefault();
    e.stopPropagation();

    const scanCode = keyCodeToScanCode[e.keyCode] || e.keyCode;
    // @ts-ignore - Wails binding
    window.go.main.App.ViewerKeyboard(scanCode, false);
  };

  const handleContextMenu = (e: React.MouseEvent) => {
    e.preventDefault();
  };

  const handleDisconnect = async () => {
    try {
      // @ts-ignore - Wails binding
      await window.go.main.App.StopViewer();
    } catch (err) {
      console.error('Failed to stop viewer:', err);
    }
    onDisconnect();
  };

  return (
    <div className="rdp-viewer" ref={containerRef}>
      <div className="viewer-toolbar">
        <div className="viewer-status">
          {connecting && !connected && (
            <span className="status-connecting">
              <span className="spinner"></span>
              Connecting...
            </span>
          )}
          {connected && (
            <span className="status-connected">Connected</span>
          )}
          {error && !connecting && !connected && (
            <span className="status-error">Error</span>
          )}
        </div>
        <div className="viewer-info">
          <span className="viewer-resolution">1280x800</span>
        </div>
        <div className="viewer-actions">
          <button onClick={handleDisconnect} className="btn btn-danger btn-sm">
            Disconnect
          </button>
        </div>
      </div>

      {error && !connected && (
        <div className="viewer-error">
          <div className="error-message">{error}</div>
          <button onClick={handleDisconnect} className="btn btn-secondary">
            Go Back
          </button>
        </div>
      )}

      {(!error || connected) && (
        <div className="viewer-canvas-container">
          <canvas
            ref={canvasRef}
            width={1280}
            height={800}
            className="rdp-canvas"
            tabIndex={0}
            style={{ transform: `scale(${scale})`, transformOrigin: 'center center' }}
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
