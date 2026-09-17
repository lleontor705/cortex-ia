import { useState, useEffect, useRef, useMemo, useCallback } from 'preact/hooks';
import {
  IconZoomIn,
  IconZoomOut,
  IconMaximize,
  IconLayout,
  IconCheck,
  IconAlert,
  IconClock
} from './icons.jsx';

const NODE_WIDTH = 270;
const NODE_HEIGHT = 150;
const LAYER_X_SPACING = 340;
const NODE_Y_SPACING = 190;

const safeStatus = value => /^[a-z_]+$/.test(value || '') ? value : 'unknown';

const statusColors = {
  done: '#10b981',
  in_progress: '#8b5cf6',
  in_review: '#6366f1',
  ready: '#06b6d4',
  blocked: '#f43f5e',
  backlog: '#64748b',
  superseded: '#475569'
};

const statusLabels = {
  done: 'Completada',
  in_progress: 'En progreso',
  in_review: 'En revisión',
  ready: 'Lista',
  blocked: 'Bloqueada',
  backlog: 'Backlog',
  superseded: 'Descompuesta'
};

const isLive = value => value && new Date(value).getTime() > Date.now();

// Computes a hierarchical DAG layout
function computeDagLayout(items) {
  if (!items || items.length === 0) return {};

  const itemsMap = new Map(items.map(it => [it.task_id, it]));
  const inDegree = new Map();
  const childrenMap = new Map();

  items.forEach(it => {
    inDegree.set(it.task_id, 0);
    childrenMap.set(it.task_id, []);
  });

  items.forEach(it => {
    const deps = (it.dependencies || []).filter(depId => itemsMap.has(depId));
    inDegree.set(it.task_id, deps.length);
    deps.forEach(depId => {
      childrenMap.get(depId)?.push(it.task_id);
    });
  });

  // Calculate layer / depth for each task
  const depth = new Map();

  function getDepth(id, path = new Set()) {
    if (path.has(id)) return 0; // Cycle safety
    if (depth.has(id)) return depth.get(id);

    const it = itemsMap.get(id);
    const deps = (it?.dependencies || []).filter(depId => itemsMap.has(depId));
    if (deps.length === 0) {
      depth.set(id, 0);
      return 0;
    }

    path.add(id);
    let maxDepDepth = 0;
    for (const depId of deps) {
      maxDepDepth = Math.max(maxDepDepth, getDepth(depId, new Set(path)));
    }
    path.delete(id);

    const d = maxDepDepth + 1;
    depth.set(id, d);
    return d;
  }

  items.forEach(it => getDepth(it.task_id));

  // Group nodes by layer
  const layers = new Map();
  items.forEach(it => {
    const d = depth.get(it.task_id) || 0;
    if (!layers.has(d)) layers.set(d, []);
    layers.get(d).push(it);
  });

  // Calculate layout coordinates
  const positions = {};
  
  // Find max layer height to center smaller layers vertically
  let maxCount = 0;
  layers.forEach(nodes => {
    if (nodes.length > maxCount) maxCount = nodes.length;
  });
  const maxTotalHeight = maxCount * NODE_Y_SPACING;

  layers.forEach((nodes, layerIdx) => {
    const layerHeight = nodes.length * NODE_Y_SPACING;
    const yOffset = (maxTotalHeight - layerHeight) / 2;

    nodes.forEach((it, idx) => {
      positions[it.task_id] = {
        x: 60 + layerIdx * LAYER_X_SPACING,
        y: 60 + yOffset + idx * NODE_Y_SPACING
      };
    });
  });

  return positions;
}

export function FlowCanvas({
  items = [],
  itemsByID = new Map(),
  boardId = 'default',
  onTask = () => {},
  onNewTask = null
}) {
  const containerRef = useRef(null);
  const [viewport, setViewport] = useState({ x: 40, y: 40, zoom: 0.85 });
  const [positions, setPositions] = useState({});
  const [hoveredNodeId, setHoveredNodeId] = useState(null);
  const [draggingNode, setDraggingNode] = useState(null); // { id, startPos, startMouse, hasMoved }
  const [panning, setPanning] = useState(null); // { startViewport, startMouse }

  const storageKey = `cortex_flow_layout_${boardId}`;

  // Initial layout calculation / restore from localStorage
  useEffect(() => {
    if (!items || items.length === 0) return;

    let saved = null;
    try {
      const raw = localStorage.getItem(storageKey);
      if (raw) saved = JSON.parse(raw);
    } catch (_) {}

    const defaultLayout = computeDagLayout(items);

    if (saved && typeof saved === 'object') {
      const merged = { ...defaultLayout };
      items.forEach(it => {
        if (saved[it.task_id] && typeof saved[it.task_id].x === 'number') {
          merged[it.task_id] = saved[it.task_id];
        }
      });
      setPositions(merged);
    } else {
      setPositions(defaultLayout);
    }
  }, [items, boardId]);

  // Persist positions when updated
  const savePositions = useCallback((newPositions) => {
    try {
      localStorage.setItem(storageKey, JSON.stringify(newPositions));
    } catch (_) {}
  }, [storageKey]);

  // Auto-layout action
  const handleAutoLayout = useCallback(() => {
    const layout = computeDagLayout(items);
    setPositions(layout);
    savePositions(layout);
    handleFitView(layout);
  }, [items, savePositions]);

  // Fit view calculation
  const handleFitView = useCallback((customPositions = null) => {
    const pos = customPositions || positions;
    const ids = Object.keys(pos);
    if (ids.length === 0 || !containerRef.current) return;

    const rect = containerRef.current.getBoundingClientRect();
    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;

    ids.forEach(id => {
      const p = pos[id];
      if (!p) return;
      if (p.x < minX) minX = p.x;
      if (p.x + NODE_WIDTH > maxX) maxX = p.x + NODE_WIDTH;
      if (p.y < minY) minY = p.y;
      if (p.y + NODE_HEIGHT > maxY) maxY = p.y + NODE_HEIGHT;
    });

    const graphWidth = Math.max(100, maxX - minX);
    const graphHeight = Math.max(100, maxY - minY);
    const pad = 80;

    const scaleX = (rect.width - pad * 2) / graphWidth;
    const scaleY = (rect.height - pad * 2) / graphHeight;
    const fitZoom = Math.max(0.3, Math.min(1.15, Math.min(scaleX, scaleY)));

    const centerX = minX + graphWidth / 2;
    const centerY = minY + graphHeight / 2;

    setViewport({
      x: rect.width / 2 - centerX * fitZoom,
      y: rect.height / 2 - centerY * fitZoom,
      zoom: fitZoom
    });
  }, [positions]);

  // Zoom helpers
  const handleZoom = useCallback((delta, clientX = null, clientY = null) => {
    if (!containerRef.current) return;
    const rect = containerRef.current.getBoundingClientRect();

    const mouseX = clientX !== null ? clientX - rect.left : rect.width / 2;
    const mouseY = clientY !== null ? clientY - rect.top : rect.height / 2;

    setViewport(prev => {
      const newZoom = Math.max(0.2, Math.min(2.0, prev.zoom * (1 + delta)));
      if (Math.abs(newZoom - prev.zoom) < 0.001) return prev;

      // Keep point under cursor invariant
      const canvasX = (mouseX - prev.x) / prev.zoom;
      const canvasY = (mouseY - prev.y) / prev.zoom;

      return {
        zoom: newZoom,
        x: mouseX - canvasX * newZoom,
        y: mouseY - canvasY * newZoom
      };
    });
  }, []);

  // Wheel zoom
  const onWheel = useCallback((e) => {
    e.preventDefault();
    const delta = -e.deltaY * 0.0015;
    handleZoom(delta, e.clientX, e.clientY);
  }, [handleZoom]);

  // Canvas Pan Handlers
  const onCanvasPointerDown = useCallback((e) => {
    if (e.target.closest('.flow-node-card') || e.target.closest('.flow-controls-dock') || e.target.closest('.flow-minimap')) {
      return;
    }
    if (e.button !== 0 && e.button !== 1) return;

    e.preventDefault();
    setPanning({
      startViewport: { ...viewport },
      startMouse: { x: e.clientX, y: e.clientY }
    });
  }, [viewport]);

  // Node Drag Handlers
  const onNodePointerDown = useCallback((e, taskId) => {
    if (e.button !== 0) return;
    e.stopPropagation();

    const nodePos = positions[taskId] || { x: 0, y: 0 };
    setDraggingNode({
      id: taskId,
      startPos: { ...nodePos },
      startMouse: { x: e.clientX, y: e.clientY },
      hasMoved: false
    });
  }, [positions]);

  // Window Pointer Move & Up for smooth dragging/panning
  useEffect(() => {
    const handlePointerMove = (e) => {
      if (draggingNode) {
        const dx = (e.clientX - draggingNode.startMouse.x) / viewport.zoom;
        const dy = (e.clientY - draggingNode.startMouse.y) / viewport.zoom;

        if (!draggingNode.hasMoved && (Math.abs(dx) > 3 || Math.abs(dy) > 3)) {
          setDraggingNode(prev => prev ? { ...prev, hasMoved: true } : null);
        }

        setPositions(prev => ({
          ...prev,
          [draggingNode.id]: {
            x: Math.round(draggingNode.startPos.x + dx),
            y: Math.round(draggingNode.startPos.y + dy)
          }
        }));
      } else if (panning) {
        const dx = e.clientX - panning.startMouse.x;
        const dy = e.clientY - panning.startMouse.y;
        setViewport({
          ...panning.startViewport,
          x: panning.startViewport.x + dx,
          y: panning.startViewport.y + dy
        });
      }
    };

    const handlePointerUp = () => {
      if (draggingNode) {
        if (draggingNode.hasMoved) {
          savePositions(positions);
        }
        setDraggingNode(null);
      }
      if (panning) {
        setPanning(null);
      }
    };

    window.addEventListener('pointermove', handlePointerMove);
    window.addEventListener('pointerup', handlePointerUp);
    return () => {
      window.removeEventListener('pointermove', handlePointerMove);
      window.removeEventListener('pointerup', handlePointerUp);
    };
  }, [draggingNode, panning, viewport, positions, savePositions]);

  // Compute connected graph for hover highlights
  const connectedNodes = useMemo(() => {
    if (!hoveredNodeId) return null;
    const connected = new Set([hoveredNodeId]);

    // Upstream ancestors (prerequisites)
    function addAncestors(id) {
      const it = itemsByID.get(id);
      (it?.dependencies || []).forEach(depId => {
        if (!connected.has(depId)) {
          connected.add(depId);
          addAncestors(depId);
        }
      });
    }

    // Downstream dependents (children)
    function addDescendants(id) {
      items.forEach(it => {
        if ((it.dependencies || []).includes(id) && !connected.has(it.task_id)) {
          connected.add(it.task_id);
          addDescendants(it.task_id);
        }
      });
    }

    addAncestors(hoveredNodeId);
    addDescendants(hoveredNodeId);
    return connected;
  }, [hoveredNodeId, items, itemsByID]);

  // Compute edges / Bézier cables
  const edges = useMemo(() => {
    const list = [];
    items.forEach(targetItem => {
      const targetPos = positions[targetItem.task_id];
      if (!targetPos) return;

      (targetItem.dependencies || []).forEach(depId => {
        const sourceItem = itemsByID.get(depId);
        const sourcePos = positions[depId];
        if (!sourcePos) return;

        // Output port from right edge of source
        const x1 = sourcePos.x + NODE_WIDTH;
        const y1 = sourcePos.y + NODE_HEIGHT / 2;

        // Input port to left edge of target
        const x2 = targetPos.x;
        const y2 = targetPos.y + NODE_HEIGHT / 2;

        const dx = Math.max(60, Math.abs(x2 - x1) * 0.45);
        const path = `M ${x1} ${y1} C ${x1 + dx} ${y1}, ${x2 - dx} ${y2}, ${x2} ${y2}`;

        const isConnected = connectedNodes ? (connectedNodes.has(depId) && connectedNodes.has(targetItem.task_id)) : false;
        const isDimmed = connectedNodes ? !isConnected : false;

        const isResolved = sourceItem?.status === 'done';
        const isBlocked = targetItem.status === 'blocked' || sourceItem?.status === 'blocked';
        const isActive = !isResolved && (targetItem.status === 'in_progress' || targetItem.status === 'ready');

        list.push({
          id: `${depId}->${targetItem.task_id}`,
          sourceId: depId,
          targetId: targetItem.task_id,
          path,
          x1, y1, x2, y2,
          isResolved,
          isBlocked,
          isActive,
          isHighlighted: isConnected,
          isDimmed
        });
      });
    });
    return list;
  }, [items, positions, itemsByID, connectedNodes]);

  // Minimap bounds and rendering
  const minimapData = useMemo(() => {
    const ids = Object.keys(positions);
    if (ids.length === 0) return null;

    let minX = Infinity, maxX = -Infinity, minY = Infinity, maxY = -Infinity;
    ids.forEach(id => {
      const p = positions[id];
      if (!p) return;
      if (p.x < minX) minX = p.x;
      if (p.x + NODE_WIDTH > maxX) maxX = p.x + NODE_WIDTH;
      if (p.y < minY) minY = p.y;
      if (p.y + NODE_HEIGHT > maxY) maxY = p.y + NODE_HEIGHT;
    });

    const pad = 100;
    minX -= pad;
    maxX += pad;
    minY -= pad;
    maxY += pad;

    const width = Math.max(200, maxX - minX);
    const height = Math.max(150, maxY - minY);

    return { minX, minY, width, height };
  }, [positions]);

  return (
    <div
      ref={containerRef}
      class="flow-canvas-container"
      onWheel={onWheel}
      onPointerDown={onCanvasPointerDown}
    >
      {/* Interactive Canvas Plane */}
      <div
        class="flow-viewport"
        style={{
          transform: `translate(${viewport.x}px, ${viewport.y}px) scale(${viewport.zoom})`
        }}
      >
        {/* SVG Cable Layer */}
        <svg class="flow-edges-layer" style={{ width: '100%', height: '100%' }}>
          <defs>
            <marker id="flow-arrow-done" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="#10b981" />
            </marker>
            <marker id="flow-arrow-active" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="#8b5cf6" />
            </marker>
            <marker id="flow-arrow-ready" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="#06b6d4" />
            </marker>
            <marker id="flow-arrow-blocked" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="#f43f5e" />
            </marker>
            <marker id="flow-arrow-default" markerWidth="8" markerHeight="8" refX="7" refY="4" orient="auto">
              <path d="M 0 1 L 7 4 L 0 7 z" fill="#475569" />
            </marker>
          </defs>

          {edges.map(edge => {
            let strokeColor = '#475569';
            let markerId = 'flow-arrow-default';

            if (edge.isBlocked) {
              strokeColor = '#f43f5e';
              markerId = 'flow-arrow-blocked';
            } else if (edge.isResolved) {
              strokeColor = '#10b981';
              markerId = 'flow-arrow-done';
            } else if (edge.isActive) {
              strokeColor = '#8b5cf6';
              markerId = 'flow-arrow-active';
            }

            const classes = [
              'flow-cable',
              edge.isResolved ? 'cable-resolved' : '',
              edge.isActive ? 'cable-active' : '',
              edge.isBlocked ? 'cable-blocked' : '',
              edge.isHighlighted ? 'cable-highlight' : '',
              edge.isDimmed ? 'cable-dimmed' : ''
            ].filter(Boolean).join(' ');

            return (
              <g key={edge.id} class={classes}>
                {/* Wider invisible stroke for easier hover */}
                <path
                  d={edge.path}
                  fill="none"
                  stroke="transparent"
                  stroke-width="18"
                  class="flow-cable-hitbox"
                  onMouseEnter={() => setHoveredNodeId(edge.sourceId)}
                  onMouseLeave={() => setHoveredNodeId(null)}
                />
                {/* Visible Bézier cable */}
                <path
                  d={edge.path}
                  fill="none"
                  stroke={strokeColor}
                  stroke-width={edge.isHighlighted ? '3.5' : '2'}
                  marker-end={`url(#${markerId})`}
                  class="flow-cable-path"
                />
              </g>
            );
          })}
        </svg>

        {/* Task Nodes Layer */}
        <div class="flow-nodes-layer">
          {items.map(item => {
            const pos = positions[item.task_id] || { x: 40, y: 40 };
            const pendingDeps = (item.dependencies || []).filter(id => itemsByID.get(id)?.status !== 'done');
            const liveClaim = isLive(item.claim?.expires_at);
            const isHighlighted = connectedNodes ? connectedNodes.has(item.task_id) : false;
            const isDimmed = connectedNodes ? !isHighlighted : false;
            const isSelected = hoveredNodeId === item.task_id;

            const cardClasses = [
              'flow-node-card',
              item.status,
              liveClaim ? 'has-claim' : '',
              isSelected ? 'selected' : '',
              isHighlighted ? 'highlighted' : '',
              isDimmed ? 'dimmed' : ''
            ].filter(Boolean).join(' ');

            return (
              <div
                key={item.task_id}
                class={cardClasses}
                style={{
                  transform: `translate3d(${pos.x}px, ${pos.y}px, 0px)`,
                  width: `${NODE_WIDTH}px`
                }}
                onPointerDown={(e) => onNodePointerDown(e, item.task_id)}
                onMouseEnter={() => setHoveredNodeId(item.task_id)}
                onMouseLeave={() => setHoveredNodeId(null)}
                onClick={() => {
                  if (!draggingNode?.hasMoved) {
                    onTask(item);
                  }
                }}
              >
                {/* Port In (Prerequisites) */}
                {(item.dependencies?.length || 0) > 0 && (
                  <div
                    class={`flow-port flow-port-in ${pendingDeps.length === 0 ? 'resolved' : 'pending'}`}
                    title={`${item.dependencies.length} dependencias (${pendingDeps.length} pendientes)`}
                  >
                    <span class="port-dot"></span>
                  </div>
                )}

                {/* Node Header */}
                <div class="flow-node-header">
                  <div class="flow-node-badge">
                    <code>{item.task_id}</code>
                    <span class="flow-rev-pill">r{item.revision}</span>
                  </div>
                  <span class={`status-chip ${safeStatus(item.status)} mini`}>
                    {statusLabels[item.status] || item.status}
                  </span>
                </div>

                {/* Node Title & Objective */}
                <div class="flow-node-body">
                  <h4 class="flow-node-title" title={item.title}>{item.title}</h4>
                  <p class="flow-node-objective" title={item.objective}>
                    {item.objective || 'Objetivo detallado pendiente.'}
                  </p>
                </div>

                {/* Node Meta Signals */}
                <div class="flow-node-footer">
                  <div class="flow-signal-group">
                    {liveClaim ? (
                      <span class="flow-signal active" title={`Claim activo por ${item.claim.owner}`}>
                        <span class="signal-dot pulsing"></span>
                        <b>{item.claim.owner}</b>
                      </span>
                    ) : (
                      <span class="flow-signal muted">
                        <span class="signal-dot"></span>
                        <span>sin claim</span>
                      </span>
                    )}
                  </div>

                  <div class="flow-badges-group">
                    {(item.allowed_files?.length || item.leases?.length || 0) > 0 && (
                      <span class="flow-pill files" title="Archivos previstos / reservados">
                        {item.allowed_files?.length || item.leases?.length} arch.
                      </span>
                    )}
                    {item.dependencies?.length > 0 && (
                      <span
                        class={`flow-pill deps ${pendingDeps.length === 0 ? 'resolved' : 'pending'}`}
                        title={`${pendingDeps.length} dependencias pendientes de ${item.dependencies.length}`}
                      >
                        {pendingDeps.length === 0 ? '✓' : `${pendingDeps.length} dep`}
                      </span>
                    )}
                  </div>
                </div>

                {/* Port Out (Dependents) */}
                <div class="flow-port flow-port-out" title="Salida hacia tareas dependientes">
                  <span class="port-dot"></span>
                </div>
              </div>
            );
          })}
        </div>
      </div>

      {/* Floating Canvas Controls Dock */}
      <div class="flow-controls-dock">
        <button
          class="flow-btn"
          onClick={() => handleZoom(0.2)}
          title="Acercar (Zoom In)"
          aria-label="Acercar"
        >
          <IconZoomIn size={16} />
        </button>
        <button
          class="flow-btn"
          onClick={() => handleZoom(-0.2)}
          title="Alejar (Zoom Out)"
          aria-label="Alejar"
        >
          <IconZoomOut size={16} />
        </button>
        <button
          class="flow-btn"
          onClick={() => handleFitView()}
          title="Ajustar a pantalla (Fit View)"
          aria-label="Ajustar vista"
        >
          <IconMaximize size={16} />
        </button>
        <button
          class="flow-btn"
          onClick={handleAutoLayout}
          title="Reorganizar DAG automáticamente"
          aria-label="Reorganizar grafo"
        >
          <IconLayout size={16} />
        </button>
        <span class="flow-zoom-display">{Math.round(viewport.zoom * 100)}%</span>
      </div>

      {/* Minimap (Radar) */}
      {minimapData && (
        <div
          class="flow-minimap"
          onClick={(e) => {
            if (!containerRef.current) return;
            const minimapRect = e.currentTarget.getBoundingClientRect();
            const clickX = (e.clientX - minimapRect.left) / minimapRect.width;
            const clickY = (e.clientY - minimapRect.top) / minimapRect.height;

            const targetCanvasX = minimapData.minX + clickX * minimapData.width;
            const targetCanvasY = minimapData.minY + clickY * minimapData.height;

            const containerRect = containerRef.current.getBoundingClientRect();
            setViewport(prev => ({
              ...prev,
              x: containerRect.width / 2 - targetCanvasX * prev.zoom,
              y: containerRect.height / 2 - targetCanvasY * prev.zoom
            }));
          }}
        >
          <svg
            viewBox={`${minimapData.minX} ${minimapData.minY} ${minimapData.width} ${minimapData.height}`}
            class="minimap-svg"
          >
            {/* Mini Nodes */}
            {items.map(item => {
              const p = positions[item.task_id];
              if (!p) return null;
              const color = statusColors[item.status] || '#64748b';
              return (
                <rect
                  key={item.task_id}
                  x={p.x}
                  y={p.y}
                  width={NODE_WIDTH}
                  height={NODE_HEIGHT}
                  rx="18"
                  fill={color}
                  opacity={hoveredNodeId === item.task_id ? 1 : 0.65}
                />
              );
            })}

            {/* Viewport Box */}
            {containerRef.current && (
              <rect
                x={-viewport.x / viewport.zoom}
                y={-viewport.y / viewport.zoom}
                width={containerRef.current.clientWidth / viewport.zoom}
                height={containerRef.current.clientHeight / viewport.zoom}
                fill="rgba(139, 92, 246, 0.12)"
                stroke="#8b5cf6"
                stroke-width={4 / viewport.zoom}
                rx="6"
              />
            )}
          </svg>
          <span class="minimap-tag">DAG RADAR</span>
        </div>
      )}
    </div>
  );
}
