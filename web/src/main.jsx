import { render } from 'preact';
import { useCallback, useEffect, useMemo, useState } from 'preact/hooks';
import './styles.css';

const states = [
  ['backlog', 'Backlog'],
  ['ready', 'Lista'],
  ['in_progress', 'En progreso'],
  ['in_review', 'En revisión'],
  ['blocked', 'Bloqueada'],
  ['superseded', 'Descompuesta'],
  ['done', 'Completada']
];
const stateLabels = Object.fromEntries(states);
const emptyDashboard = { summary: {}, sessions: [], active_work: [], delegations: [], activity: [] };
const safeStatus = value => /^[a-z_]+$/.test(value || '') ? value : 'unknown';
const shortID = value => value?.length > 16 ? `${value.slice(0, 8)}…${value.slice(-5)}` : value || '—';
const formatTime = value => value ? new Intl.DateTimeFormat('es-CO', { dateStyle: 'medium', timeStyle: 'short' }).format(new Date(value)) : '—';
const relativeTime = value => {
  if (!value) return '—';
  const seconds = Math.round((new Date(value).getTime() - Date.now()) / 1000);
  const abs = Math.abs(seconds);
  const [amount, unit] = abs < 60 ? [seconds, 'second'] : abs < 3600 ? [Math.round(seconds / 60), 'minute'] : abs < 86400 ? [Math.round(seconds / 3600), 'hour'] : [Math.round(seconds / 86400), 'day'];
  return new Intl.RelativeTimeFormat('es', { numeric: 'auto' }).format(amount, unit);
};

async function request(path, options = {}) {
  const response = await fetch(path, options);
  const body = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(body.error || `Error HTTP ${response.status}`);
  return body;
}

function StatusChip({ status, label }) {
  return <span class={`status-chip ${safeStatus(status)}`}>{label || stateLabels[status] || status}</span>;
}

function Empty({ title, children }) {
  return <div class="empty-inline"><b>{title}</b><span>{children}</span></div>;
}

function PageHeader({ eyebrow, title, description, action }) {
  return (
    <header class="page-header">
      <div>
        <p class="eyebrow">{eyebrow}</p>
        <h1>{title}</h1>
        <p>{description}</p>
      </div>
      {action && <div class="page-header-action">{action}</div>}
    </header>
  );
}

function Sidebar({ boards, dashboard, currentBoard, view, onView, onBoard, onNewBoard }) {
  const nav = [
    ['overview', '⌂', 'Resumen'],
    ['sessions', '◎', 'Sesiones'],
    ['delegations', '⇄', 'Delegación'],
    ['activity', '↯', 'Actividad'],
    ['settings', '⚙', 'Ecosistema']
  ];
  return (
    <aside class="sidebar">
      <button class="brand" onClick={() => onView('overview')}>
        <span class="brand-mark">C</span>
        <span>
          <strong>Cortex-IA</strong>
          <small>Operations Console</small>
        </span>
      </button>
      <p class="side-label">CONTROL</p>
      <nav class="primary-nav" aria-label="Navegación principal">
        {nav.map(([id, icon, label]) => (
          <button key={id} class={`nav-link ${view === id ? 'active' : ''}`} onClick={() => onView(id)}>
            <span class="nav-icon">{icon}</span>
            <span class="nav-text">{label}</span>
            {id === 'sessions' && <b>{dashboard.summary.sessions || 0}</b>}
            {id === 'delegations' && <b>{dashboard.summary.active_delegations || 0}</b>}
          </button>
        ))}
      </nav>
      <div class="board-heading">
        <p class="side-label">TASK BOARDS</p>
        <button onClick={onNewBoard} aria-label="Crear tablero" title="Nueva sesión / tablero">+</button>
      </div>
      <nav class="board-nav" aria-label="Tableros">
        {boards.length ? boards.map(board => (
          <button
            key={board.board_id}
            class={`board-link ${view === 'board' && currentBoard === board.board_id ? 'active' : ''}`}
            onClick={() => onBoard(board.board_id)}
          >
            <span>{board.title}</span>
            <b>{Object.values(board.counts || {}).reduce((a, b) => a + b, 0)}</b>
          </button>
        )) : <small class="empty-boards">Sin tableros activos</small>}
      </nav>
      <footer>
        <div>
          <span class="online-dot"></span>
          <strong>Control Local Activo</strong>
        </div>
        <small>SQLite · loopback seguro</small>
      </footer>
    </aside>
  );
}

function Metrics({ dashboard }) {
  const s = dashboard.summary;
  return (
    <div class="metrics five">
      <article>
        <span>Sesiones</span>
        <strong>{s.sessions || 0}</strong>
        <small>boards durables</small>
      </article>
      <article>
        <span>Trabajo activo</span>
        <strong>{s.active_tasks || 0}</strong>
        <small>tareas no cerradas</small>
      </article>
      <article>
        <span>Agentes activos</span>
        <strong>{s.active_agents || 0}</strong>
        <small>claims vigentes</small>
      </article>
      <article>
        <span>Delegaciones</span>
        <strong>{s.active_delegations || 0}</strong>
        <small>ejecutores activos</small>
      </article>
      <article class="metric-alert">
        <span>Bloqueadas</span>
        <strong>{s.blocked || 0}</strong>
        <small>requieren atención</small>
      </article>
    </div>
  );
}

function WorkRow({ item, onOpen }) {
  return (
    <button class="work-row" onClick={() => onOpen(item)}>
      <span class={`status-dot ${safeStatus(item.status)}`}></span>
      <span class="work-copy">
        <b>{item.title}</b>
        <small>{item.task_id} · {item.board_id}</small>
      </span>
      <span class="owner-chip">{item.claim?.owner || 'Sin claim'}</span>
      <span class="lease-count">{item.leases?.length || 0} leases</span>
      <StatusChip status={item.status} />
    </button>
  );
}

function EventList({ events, compact = false }) {
  if (!events.length) return <Empty title="Sin actividad">Los eventos de trabajo y delegación aparecerán aquí.</Empty>;
  return (
    <div class={`timeline ${compact ? 'compact' : ''}`}>
      {events.map((event, index) => (
        <article class="event" key={`${event.source}-${event.entity_id}-${event.created_at}-${index}`}>
          <span class={`event-mark ${event.source}`}>{event.source === 'work' ? 'W' : 'D'}</span>
          <div>
            <header>
              <b>{event.kind.replaceAll('_', ' ')}</b>
              <time title={formatTime(event.created_at)}>{relativeTime(event.created_at)}</time>
            </header>
            <p>{event.title} <code>{shortID(event.entity_id)}</code></p>
            {(event.from || event.to) && <small class="event-diff">{event.from || '—'} → {event.to || '—'}</small>}
            {event.detail && <small class="event-detail">{event.detail}</small>}
          </div>
        </article>
      ))}
    </div>
  );
}

function JobRows({ jobs, preview = false, onOpen }) {
  if (!jobs.length) return <Empty title="Sin delegaciones">Las ejecuciones delegadas a CLI aparecerán aquí.</Empty>;
  if (preview) {
    return (
      <div class="job-list">
        {jobs.slice(0, 5).map(job => (
          <button class="job-row-btn" key={job.job_id} onClick={() => onOpen?.(job)}>
            <span class="transport-mark">{job.transport === 'herdr' ? '▦' : '⌁'}</span>
            <span class="job-copy">
              <b>{job.role}</b>
              <small>{job.task_id || shortID(job.job_id)}</small>
            </span>
            <StatusChip status={job.status} />
            <time>{relativeTime(job.updated_at)}</time>
          </button>
        ))}
      </div>
    );
  }
  return (
    <>
      {jobs.map(job => (
        <tr key={job.job_id} class="clickable-tr" onClick={() => onOpen?.(job)}>
          <td><code title={job.job_id}>{shortID(job.job_id)}</code></td>
          <td>
            <b>{job.role}</b>
            <small>{job.task_id || 'sin task vinculada'}</small>
          </td>
          <td>
            <span class={`transport-badge ${job.transport}`}>
              {job.transport === 'herdr' ? '▦ Herdr' : '⌁ Direct CLI'}
            </span>
          </td>
          <td><code>{job.lease_owner ? shortID(job.lease_owner) : '—'}</code></td>
          <td>#{job.attempt}</td>
          <td>
            <StatusChip status={job.status} />
            {job.error_code && <small class="error-copy">{job.error_code}</small>}
          </td>
          <td title={formatTime(job.updated_at)}>{relativeTime(job.updated_at)}</td>
        </tr>
      ))}
    </>
  );
}

function Overview({ dashboard, onView, onTask, onNewTask, onJob }) {
  return (
    <>
      <PageHeader
        eyebrow="SYSTEM OVERVIEW"
        title="Operaciones en Tiempo Real"
        description="Sesiones de trabajo, agentes, tareas y ejecuciones delegadas desde el plano de control local."
        action={<button class="button primary" onClick={onNewTask}>+ Nueva tarea</button>}
      />
      <Metrics dashboard={dashboard} />
      <div class="dashboard-grid">
        <section class="panel work-panel">
          <header>
            <div>
              <p class="eyebrow">ACTIVE WORK</p>
              <h2>Trabajo en curso</h2>
            </div>
            <button class="text-button" onClick={() => onView('sessions')}>Ver sesiones →</button>
          </header>
          <div class="work-list">
            {dashboard.active_work.length ? (
              dashboard.active_work.slice(0, 8).map(item => <WorkRow key={item.task_id} item={item} onOpen={onTask} />)
            ) : (
              <Empty title="No hay tareas activas">Crea una sesión o tarea para comenzar.</Empty>
            )}
          </div>
        </section>
        <section class="panel activity-panel">
          <header>
            <div>
              <p class="eyebrow">EVENT STREAM</p>
              <h2>Actividad reciente</h2>
            </div>
            <button class="text-button" onClick={() => onView('activity')}>Ver todo →</button>
          </header>
          <EventList events={dashboard.activity.slice(0, 7)} compact />
        </section>
        <section class="panel delegation-panel">
          <header>
            <div>
              <p class="eyebrow">EXECUTION BRIDGE</p>
              <h2>Delegación externa</h2>
            </div>
            <button class="text-button" onClick={() => onView('delegations')}>Abrir monitor →</button>
          </header>
          <JobRows jobs={dashboard.delegations} preview onOpen={onJob} />
        </section>
      </div>
    </>
  );
}

function Sessions({ sessions, onBoard, onNew }) {
  return (
    <>
      <PageHeader
        eyebrow="WORK SESSIONS"
        title="Sesiones Coordinadas"
        description="Cada task board representa una sesión durable con su DAG de dependencias, progreso y leases."
        action={<button class="button primary" onClick={onNew}>+ Nueva sesión</button>}
      />
      <div class="session-grid">
        {sessions.length ? (
          sessions.map(session => (
            <button class="session-card" key={session.board_id} onClick={() => onBoard(session.board_id)}>
              <header>
                <StatusChip status={session.status} label={session.status === 'complete' ? 'Completada' : session.status === 'active' ? 'Activa' : 'Vacía'} />
                <small>{relativeTime(session.updated_at)}</small>
              </header>
              <h2>{session.title}</h2>
              <p>{session.description || `Sesión ${session.board_id}`}</p>
              <div class="session-stats">
                <span><b>{session.task_count}</b> tareas</span>
                <span><b>{session.owners?.length || 0}</b> agentes</span>
                <span><b>{session.counts?.blocked || 0}</b> bloqueadas</span>
              </div>
              <progress max="100" value={session.progress || 0} aria-label={`${session.progress || 0}% completado`}></progress>
              <footer>
                <span>{session.progress || 0}% completado</span>
                <span>rev. durable →</span>
              </footer>
            </button>
          ))
        ) : (
          <Empty title="No hay sesiones">Crea un task board para iniciar una sesión coordinada.</Empty>
        )}
      </div>
    </>
  );
}

function Delegations({ jobs, onOpenJob }) {
  const [filter, setFilter] = useState('');
  const filtered = useMemo(() => {
    if (!filter) return jobs;
    const q = filter.toLowerCase();
    return jobs.filter(j => j.role?.toLowerCase().includes(q) || j.task_id?.toLowerCase().includes(q) || j.status?.toLowerCase().includes(q) || j.job_id?.toLowerCase().includes(q));
  }, [jobs, filter]);

  return (
    <>
      <PageHeader
        eyebrow="DELEGATION MONITOR"
        title="Supervisión de Tareas Delegadas"
        description="Monitoreo transparente de tareas ejecutadas por workers CLI y multiplexores terminales en tiempo real."
        action={
          <div class="search-box">
            <input
              type="text"
              placeholder="Filtrar por rol, task o estado..."
              value={filter}
              onInput={e => setFilter(e.currentTarget.value)}
            />
          </div>
        }
      />
      <section class="panel table-panel">
        <div class="table-scroll">
          <table>
            <thead>
              <tr>
                <th>Job ID</th>
                <th>Rol / Task</th>
                <th>Transporte</th>
                <th>Owner Claim</th>
                <th>Intento</th>
                <th>Estado</th>
                <th>Actualizado</th>
              </tr>
            </thead>
            <tbody>
              {filtered.length ? (
                <JobRows jobs={filtered} onOpen={onOpenJob} />
              ) : (
                <tr>
                  <td colspan="7">
                    <Empty title="Sin ejecuciones delegadas">
                      {filter ? 'No hay resultados para tu búsqueda.' : 'Cuando un agente delegue una tarea a un worker externo aparecerá aquí.'}
                    </Empty>
                  </td>
                </tr>
              )}
            </tbody>
          </table>
        </div>
      </section>
    </>
  );
}

function SettingsView({ configData, onRefresh }) {
  const cfg = configData?.delegation_config || {};
  const roles = cfg?.roles || {};
  const herdr = cfg?.herdr_settings || {};

  return (
    <>
      <PageHeader
        eyebrow="ECOSYSTEM & DELEGATION CONFIG"
        title="Configuración Dinámica del Ecosistema"
        description="Consulta las políticas activas de delegación, ejecutores CLI por rol y estado de los multiplexores."
        action={<button class="button secondary" onClick={onRefresh}>↻ Recargar configuración</button>}
      />
      <div class="settings-grid">
        <section class="panel settings-card">
          <header>
            <div>
              <p class="eyebrow">GLOBAL POLICY</p>
              <h2>Motor de Delegación</h2>
            </div>
            <StatusChip
              status={cfg.delegation_enabled ? 'succeeded' : 'blocked'}
              label={cfg.delegation_enabled ? 'Habilitada' : 'Deshabilitada'}
            />
          </header>
          <div class="settings-body">
            <div class="setting-row">
              <span>Modo de Transporte</span>
              <b>{cfg.use_herdr ? 'Herdr Multiplexed (Terminal Split)' : 'Direct CLI (Subproceso)'}</b>
            </div>
            <div class="setting-row">
              <span>Dirección de Split</span>
              <b><code>split {herdr.split_direction || 'right'}</code></b>
            </div>
            <div class="setting-row">
              <span>Auto Split & Close</span>
              <b>{herdr.auto_split ? 'Activado (Automático)' : 'Manual'}</b>
            </div>
            <div class="setting-row">
              <span>Timeout por Tarea</span>
              <b>{herdr.timeout_seconds || 300} segundos</b>
            </div>
          </div>
        </section>

        <section class="panel settings-card">
          <header>
            <div>
              <p class="eyebrow">ENVIRONMENT</p>
              <h2>Estado del Entorno</h2>
            </div>
            <StatusChip
              status={configData?.herdr_active ? 'succeeded' : 'ready'}
              label={configData?.herdr_active ? 'Herdr Activo' : 'Standalone'}
            />
          </header>
          <div class="settings-body">
            <div class="setting-row">
              <span>Multiplexador Herdr</span>
              <b>{configData?.herdr_installed ? 'Instalado ✓' : 'No detectado'}</b>
            </div>
            <div class="setting-row">
              <span>Sesión Herdr en Terminal</span>
              <b>{configData?.herdr_active ? 'En ejecución (Paneles interactivos)' : 'Inactivo'}</b>
            </div>
            <div class="setting-row">
              <span>Subagentes en Background</span>
              <b>{configData?.background_subagents_enabled ? 'Habilitado (OPENCODE_EXPERIMENTAL)' : 'Deshabilitado'}</b>
            </div>
            <div class="setting-row">
              <span>Base de Datos Operativa</span>
              <b><code>~/.config/opencode/delegation.db</code></b>
            </div>
          </div>
        </section>
      </div>

      <section class="panel role-matrix-panel">
        <header>
          <div>
            <p class="eyebrow">ROLE DISPATCH MATRIX</p>
            <h2>Ejecutores CLI Configurados por Rol</h2>
          </div>
        </header>
        <div class="role-grid">
          {Object.entries(roles).map(([roleName, roleCfg]) => (
            <article class="role-card" key={roleName}>
              <header>
                <div>
                  <span class="role-icon">{roleName === 'implement' ? '🛠️' : roleName === 'investigate' ? '🔍' : roleName === 'planner' ? '📋' : '🛡️'}</span>
                  <h3>role/{roleName}</h3>
                </div>
                <StatusChip
                  status={roleCfg.delegate ? 'succeeded' : 'ready'}
                  label={roleCfg.delegate ? 'Delegado' : 'Nativo'}
                />
              </header>
              <div class="role-details">
                <div class="role-field">
                  <span>CLI Ejecutor</span>
                  <b><code>{roleCfg.cli || 'native'}</code></b>
                </div>
                <div class="role-field">
                  <span>Modo Operativo</span>
                  <b><code>{roleCfg.mode || 'default'}</code></b>
                </div>
                <div class="role-field">
                  <span>Delegación Externa</span>
                  <b>{roleCfg.delegate ? 'Sí (Supervisada)' : 'No (In-process)'}</b>
                </div>
              </div>
            </article>
          ))}
        </div>
      </section>
    </>
  );
}

const isLive = value => value && new Date(value).getTime() > Date.now();

function TaskCard({ item, itemsByID, onOpen }) {
  const pendingDependencies = (item.dependencies || []).filter(id => itemsByID.get(id)?.status !== 'done');
  const liveClaim = isLive(item.claim?.expires_at);
  return (
    <button class={`task-card ${item.status}`} onClick={() => onOpen(item)}>
      <div class="task-card-topline">
        <code>{item.task_id}</code>
        <span>r{item.revision}</span>
      </div>
      <h3>{item.title}</h3>
      <p class="task-objective">{item.objective || 'Objetivo detallado pendiente de definir.'}</p>
      <div class="task-signals">
        <span class={liveClaim ? 'signal active' : 'signal'}><i></i>{liveClaim ? item.claim.owner : 'sin claim'}</span>
        <span>{item.allowed_files?.length || item.leases?.length || 0} archivos</span>
      </div>
      {(item.dependencies?.length || 0) > 0 && (
        <footer>
          <span>{pendingDependencies.length ? `${pendingDependencies.length} dependencias pendientes` : 'dependencias resueltas'}</span>
          <span>→</span>
        </footer>
      )}
    </button>
  );
}

function DependencyView({ items, itemsByID, onTask }) {
  return (
    <div class="dependency-list">
      {items.map(item => {
        const dependencies = item.dependencies || [];
        return (
          <button class="dependency-row" key={item.task_id} onClick={() => onTask(item)}>
            <span class={`dependency-state ${safeStatus(item.status)}`}></span>
            <span class="dependency-copy">
              <code>{item.task_id}</code>
              <b>{item.title}</b>
              <small>{item.objective || 'Objetivo no documentado'}</small>
            </span>
            <span class="dependency-links">
              {dependencies.length ? dependencies.map(id => (
                <span class={itemsByID.get(id)?.status === 'done' ? 'resolved' : ''} key={id}>{id}</span>
              )) : <small>raíz del DAG</small>}
            </span>
            <StatusChip status={item.status} />
          </button>
        );
      })}
    </div>
  );
}

function Board({ snapshot, activity, delegations, onNewTask, onTask }) {
  const board = snapshot?.board;
  const items = snapshot?.items || [];
  const counts = board?.counts || {};
  const [query, setQuery] = useState('');
  const [statusFilter, setStatusFilter] = useState('all');
  const [mode, setMode] = useState('flow');
  const itemsByID = useMemo(() => new Map(items.map(item => [item.task_id, item])), [items]);
  const taskIDs = useMemo(() => new Set(items.map(item => item.task_id)), [items]);
  const filteredItems = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return items.filter(item => {
      if (statusFilter !== 'all' && item.status !== statusFilter) return false;
      if (!needle) return true;
      return [item.task_id, item.title, item.objective, item.acceptance_criteria, item.verification, item.claim?.owner, ...(item.allowed_files || []), ...(item.leases || []).map(lease => lease.path)]
        .filter(Boolean)
        .some(value => value.toLowerCase().includes(needle));
    });
  }, [items, query, statusFilter]);
  const liveClaims = items.filter(item => isLive(item.claim?.expires_at));
  const leasedFiles = items.reduce((total, item) => total + (item.leases?.filter(lease => isLive(lease.expires_at)).length || 0), 0);
  const completed = (counts.done || 0) + (counts.superseded || 0);
  const progress = items.length ? Math.round((completed / items.length) * 100) : 0;
  const boardActivity = (activity || []).filter(event => taskIDs.has(event.entity_id)).slice(0, 7);
  const boardDelegations = (delegations || []).filter(job => taskIDs.has(job.task_id)).slice(0, 5);
  const attention = items.filter(item => item.status === 'blocked' || (item.status === 'in_progress' && !isLive(item.claim?.expires_at)));
  const upcoming = items
    .filter(item => ['in_progress', 'ready', 'backlog'].includes(item.status))
    .sort((a, b) => ['in_progress', 'ready', 'backlog'].indexOf(a.status) - ['in_progress', 'ready', 'backlog'].indexOf(b.status))
    .slice(0, 4);
  if (!board) return <Empty title="Cargando sesión">Consultando el task board local.</Empty>;
  return (
    <>
      <header class="board-hero">
        <div class="board-identity">
          <p class="eyebrow">DURABLE TASK BOARD · <code>{board.board_id}</code></p>
          <h1>{board.title}</h1>
          <p>{board.description || 'Sesión coordinada del plano de control local.'}</p>
          <div class="board-meta">
            <span>rev. {board.revision}</span>
            <span>{items.length} tareas</span>
            <span>actualizado {relativeTime(board.updated_at)}</span>
          </div>
        </div>
        <div class="board-progress">
          <span><b>{progress}%</b> completado</span>
          <div class="progress-track"><i style={{ width: `${progress}%` }}></i></div>
          <small>{completed} de {items.length} tareas aprobadas</small>
          <button class="button primary" onClick={onNewTask}>+ Nueva tarea</button>
        </div>
      </header>

      <section class="board-kpis" aria-label="Estado del tablero">
        <article><small>Lista para tomar</small><strong>{counts.ready || 0}</strong><span class="kpi-mark ready"></span></article>
        <article><small>Agentes con claim</small><strong>{liveClaims.length}</strong><span class="kpi-mark in_progress"></span></article>
        <article><small>Archivos reservados</small><strong>{leasedFiles}</strong><span class="kpi-mark in_review"></span></article>
        <article class={counts.blocked ? 'alert' : ''}><small>Requieren atención</small><strong>{attention.length}</strong><span class="kpi-mark blocked"></span></article>
      </section>

      {upcoming.length > 0 && <section class="execution-plan">
        <header>
          <div><p class="eyebrow">EXECUTION PLAN</p><h2>Qué se va a hacer</h2></div>
          <span>Ordenado por disponibilidad y dependencias</span>
        </header>
        <div class="plan-steps">
          {upcoming.map((item, index) => (
            <button key={item.task_id} onClick={() => onTask(item)}>
              <span class="step-number">{String(index + 1).padStart(2, '0')}</span>
              <span class="step-copy">
                <span><code>{item.task_id}</code><StatusChip status={item.status} /></span>
                <b>{item.title}</b>
                <p>{item.objective || 'Esta tarea aún no tiene un objetivo operativo documentado.'}</p>
                <small>
                  {item.allowed_files?.length ? `${item.allowed_files.length} archivos previstos` : 'alcance de archivos pendiente'}
                  {' · '}
                  {item.verification ? `verifica: ${item.verification}` : 'verificación pendiente'}
                </small>
              </span>
            </button>
          ))}
        </div>
      </section>}

      {items.length ? <div class="board-layout">
        <section class="board-workspace">
          <header class="board-toolbar">
            <div class="view-switch" aria-label="Vista del tablero">
              <button class={mode === 'flow' ? 'active' : ''} onClick={() => setMode('flow')}>Flujo</button>
              <button class={mode === 'dependencies' ? 'active' : ''} onClick={() => setMode('dependencies')}>Dependencias</button>
            </div>
            <div class="board-filters">
              <select value={statusFilter} onChange={event => setStatusFilter(event.currentTarget.value)} aria-label="Filtrar por estado">
                <option value="all">Todos los estados</option>
                {states.map(([status, label]) => <option value={status} key={status}>{label}</option>)}
              </select>
              <input value={query} onInput={event => setQuery(event.currentTarget.value)} placeholder="Buscar tarea, owner o archivo…" aria-label="Buscar tareas" />
            </div>
          </header>
          <div class="result-count">{filteredItems.length} de {items.length} tareas visibles</div>
          {mode === 'flow' ? (
            <section class="kanban">
              {states.map(([status, label]) => {
                const cards = filteredItems.filter(item => item.status === status);
                return (
                  <section class={`column ${status}`} key={status}>
                    <header class="column-head">
                      <span class={`status-dot ${status}`}></span>
                      <h2>{label}</h2>
                      <b>{cards.length}</b>
                    </header>
                    <div class="cards">
                      {cards.length ? cards.map(item => (
                        <TaskCard key={item.task_id} item={item} itemsByID={itemsByID} onOpen={onTask} />
                      )) : <span class="empty-column">Sin tareas</span>}
                    </div>
                  </section>
                );
              })}
            </section>
          ) : <DependencyView items={filteredItems} itemsByID={itemsByID} onTask={onTask} />}
        </section>

        <aside class="control-rail">
          <section class="rail-panel attention-panel">
            <header><div><p class="eyebrow">CONTROL PLANE</p><h2>Atención requerida</h2></div><b>{attention.length}</b></header>
            {attention.length ? attention.slice(0, 5).map(item => (
              <button class="rail-task" key={item.task_id} onClick={() => onTask(item)}>
                <span class={`status-dot ${safeStatus(item.status)}`}></span>
                <span><b>{item.task_id}</b><small>{item.status === 'blocked' ? 'Bloqueada' : 'Claim ausente o vencido'}</small></span>
              </button>
            )) : <Empty title="Sin alertas">Claims y bloqueos están bajo control.</Empty>}
          </section>

          <section class="rail-panel">
            <header><div><p class="eyebrow">AUTHORITY</p><h2>Claims y reservas</h2></div><b>{liveClaims.length}</b></header>
            <div class="authority-list">
              {liveClaims.length ? liveClaims.slice(0, 6).map(item => (
                <button key={item.task_id} onClick={() => onTask(item)}>
                  <span><b>{item.claim.owner}</b><small>{item.task_id}</small></span>
                  <span><strong>{item.leases?.length || 0}</strong><small>archivos</small></span>
                </button>
              )) : <Empty title="Sin claims activos">Ningún agente posee autoridad de escritura.</Empty>}
            </div>
          </section>

          <section class="rail-panel">
            <header><div><p class="eyebrow">EXTERNAL LEAVES</p><h2>Delegaciones</h2></div><b>{boardDelegations.length}</b></header>
            {boardDelegations.length ? <div class="mini-jobs">{boardDelegations.map(job => (
              <div key={job.job_id}><span>{job.role}</span><StatusChip status={job.status} /><small>{shortID(job.job_id)}</small></div>
            ))}</div> : <Empty title="Sin delegaciones">No hay jobs vinculados a este board.</Empty>}
          </section>

          <section class="rail-panel board-events">
            <header><div><p class="eyebrow">LIVE AUDIT</p><h2>Actividad reciente</h2></div></header>
            <EventList events={boardActivity} compact />
          </section>
        </aside>
      </div> : (
        <div class="empty">
          <strong>Esta sesión está lista.</strong>
          <span>Crea la primera tarea para materializar su DAG.</span>
        </div>
      )}
    </>
  );
}

function Modal({ open, onClose, children, className = '' }) {
  useEffect(() => {
    if (!open) return;
    const close = event => event.key === 'Escape' && onClose();
    document.addEventListener('keydown', close);
    return () => document.removeEventListener('keydown', close);
  }, [open, onClose]);
  if (!open) return null;
  return (
    <div class="modal-backdrop" role="presentation" onMouseDown={event => event.target === event.currentTarget && onClose()}>
      <section class={`modal ${className}`} role="dialog" aria-modal="true">
        {children}
      </section>
    </div>
  );
}

function BoardForm({ open, onClose, onCreated, onError }) {
  const submit = async event => {
    event.preventDefault();
    try {
      const data = Object.fromEntries(new FormData(event.currentTarget));
      const board = await request('/api/boards', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      });
      onCreated(board);
    } catch (error) {
      onError(error);
    }
  };
  return (
    <Modal open={open} onClose={onClose}>
      <form onSubmit={submit}>
        <header>
          <div>
            <p class="eyebrow">NEW SESSION</p>
            <h2>Crear task board</h2>
          </div>
          <button type="button" class="close" onClick={onClose}>×</button>
        </header>
        <label>
          ID estable
          <input name="board_id" required maxlength="128" placeholder="release-1" />
        </label>
        <label>
          Nombre
          <input name="title" required maxlength="256" placeholder="Release 1" />
        </label>
        <label>
          Descripción
          <textarea name="description" maxlength="2048" placeholder="Objetivo y alcance de la sesión"></textarea>
        </label>
        <div class="dialog-actions">
          <button type="button" class="button secondary" onClick={onClose}>Cancelar</button>
          <button class="button primary">Crear sesión</button>
        </div>
      </form>
    </Modal>
  );
}

function TaskForm({ open, onClose, onCreated, onError, boards, currentBoard }) {
  const submit = async event => {
    event.preventDefault();
    try {
      const data = Object.fromEntries(new FormData(event.currentTarget));
      data.dependencies = data.dependencies.split(',').map(value => value.trim()).filter(Boolean);
      data.allowed_files = data.allowed_files.split(/[\r\n,]+/).map(value => value.trim()).filter(Boolean);
      const task = await request('/api/tasks', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(data)
      });
      onCreated(task);
    } catch (error) {
      onError(error);
    }
  };
  return (
    <Modal open={open} onClose={onClose}>
      <form onSubmit={submit}>
        <header>
          <div>
            <p class="eyebrow">NEW WORK ITEM</p>
            <h2>Nueva tarea</h2>
          </div>
          <button type="button" class="close" onClick={onClose}>×</button>
        </header>
        <label>
          Tablero
          <select name="board_id" defaultValue={currentBoard} required>
            {boards.map(board => (
              <option value={board.board_id} key={board.board_id}>{board.title}</option>
            ))}
          </select>
        </label>
        <label>
          ID de tarea
          <input name="task_id" required maxlength="128" placeholder="TASK-101" />
        </label>
        <label>
          Título
          <input name="title" required maxlength="512" placeholder="Implementar el adaptador" />
        </label>
        <label>
          Qué se va a hacer
          <textarea name="objective" required maxlength="4096" placeholder="Objetivo operativo, límites y resultado esperado"></textarea>
        </label>
        <label>
          Criterios de aceptación
          <textarea name="acceptance_criteria" maxlength="4096" placeholder="Condiciones observables que deben cumplirse"></textarea>
        </label>
        <label>
          Verificación prevista
          <input name="verification" maxlength="2048" placeholder="go test ./internal/tui/..." />
        </label>
        <label>
          Archivos previstos <small>(uno por línea; rutas relativas exactas)</small>
          <textarea name="allowed_files" placeholder={'internal/example/service.go\ninternal/example/service_test.go'}></textarea>
        </label>
        <label>
          Dependencias <small>(IDs separados por coma)</small>
          <input name="dependencies" placeholder="TASK-099, TASK-100" />
        </label>
        <div class="dialog-actions">
          <button type="button" class="button secondary" onClick={onClose}>Cancelar</button>
          <button class="button primary">Crear tarea</button>
        </div>
      </form>
    </Modal>
  );
}

function TaskDetail({ item, onClose }) {
  return (
    <Modal open={!!item} onClose={onClose}>
      {item && (
        <div class="detail-content">
          <header>
            <div>
              <p class="eyebrow">{item.board_id} / {item.task_id}</p>
              <h2>{item.title}</h2>
            </div>
            <button class="close" onClick={onClose}>×</button>
          </header>
          <div class="detail-grid">
            <span>Estado<b><StatusChip status={item.status} /></b></span>
            <span>Revisión<b>{item.revision}</b></span>
            <span>Owner<b>{item.claim?.owner || 'Sin claim'}</b></span>
            <span>Expira<b>{item.claim ? relativeTime(item.claim.expires_at) : '—'}</b></span>
          </div>
          <h3>Qué se va a hacer</h3>
          <p class="definition-copy">{item.objective || 'Objetivo operativo no documentado.'}</p>
          <h3>Criterios de aceptación</h3>
          <p class="definition-copy">{item.acceptance_criteria || 'Criterios de aceptación no documentados.'}</p>
          <h3>Verificación prevista</h3>
          <div class="verification-command"><code>{item.verification || 'Comando no definido'}</code></div>
          <h3>Archivos previstos</h3>
          <div class="planned-files">
            {item.allowed_files?.length ? item.allowed_files.map(path => <code key={path}>{path}</code>) : <span>Alcance de archivos no documentado</span>}
          </div>
          <h3>Dependencias</h3>
          <div class="token-list">
            {item.dependencies?.length ? (
              item.dependencies.map(dep => <code key={dep}>{dep}</code>)
            ) : (
              <span>Ninguna</span>
            )}
          </div>
          <h3>File leases exclusivos</h3>
          <div class="lease-list">
            {item.leases?.length ? (
              item.leases.map(lease => (
                <div key={lease.path}>
                  <code>{lease.path}</code>
                  <span>expira {relativeTime(lease.expires_at)}</span>
                </div>
              ))
            ) : (
              <span>Sin archivos reservados</span>
            )}
          </div>
        </div>
      )}
    </Modal>
  );
}

function JobDetail({ job, onClose }) {
  if (!job) return null;
  const receipt = job.receipt;
  let formattedOutput = null;
  if (receipt?.output) {
    try {
      formattedOutput = JSON.stringify(typeof receipt.output === 'string' ? JSON.parse(receipt.output) : receipt.output, null, 2);
    } catch {
      formattedOutput = String(receipt.output);
    }
  }

  return (
    <Modal open={!!job} onClose={onClose} className="wide-modal">
      <div class="detail-content">
        <header>
          <div>
            <p class="eyebrow">DELEGATED JOB / {job.transport?.toUpperCase()}</p>
            <h2>Job <code>{job.job_id}</code></h2>
          </div>
          <button class="close" onClick={onClose}>×</button>
        </header>

        <div class="detail-grid">
          <span>Rol<b>{job.role}</b></span>
          <span>Estado<b><StatusChip status={job.status} /></b></span>
          <span>Task ID<b>{job.task_id || '—'}</b></span>
          <span>Transporte<b>{job.transport === 'herdr' ? '▦ Herdr Multiplexed' : '⌁ Direct CLI'}</b></span>
          <span>Intento<b>#{job.attempt}</b></span>
          <span>Exit Code<b>{receipt?.exit_code ?? '—'}</b></span>
          <span>Lease Owner<b><code>{job.lease_owner ? shortID(job.lease_owner) : '—'}</code></b></span>
          <span>Actualizado<b>{formatTime(job.updated_at)}</b></span>
        </div>

        {job.error_message && (
          <div class="job-error-box">
            <b>Error ({job.error_code || 'ERROR'}):</b>
            <pre>{job.error_message}</pre>
          </div>
        )}

        {formattedOutput && (
          <>
            <h3>Receipt Estructurado</h3>
            <pre class="code-block">{formattedOutput}</pre>
          </>
        )}

        {receipt?.output_hash && (
          <p class="hash-copy">
            <small>SHA-256 Digest: <code>{receipt.output_hash}</code></small>
          </p>
        )}
      </div>
    </Modal>
  );
}

function App() {
  const [dashboard, setDashboard] = useState(emptyDashboard);
  const [boards, setBoards] = useState([]);
  const [snapshot, setSnapshot] = useState(null);
  const [configData, setConfigData] = useState(null);
  const [view, setView] = useState(location.hash.slice(1) || 'overview');
  const [currentBoard, setCurrentBoard] = useState(localStorage.getItem('cortex-board') || 'default');
  const [boardModal, setBoardModal] = useState(false);
  const [taskModal, setTaskModal] = useState(false);
  const [detail, setDetail] = useState(null);
  const [jobDetail, setJobDetail] = useState(null);
  const [notice, setNotice] = useState(null);
  const [syncedAt, setSyncedAt] = useState(null);

  const notify = (message, error = false) => {
    setNotice({ message, error });
    setTimeout(() => setNotice(null), 2600);
  };

  const loadAll = useCallback(async (silent = true) => {
    try {
      const [nextDashboard, nextBoards, nextConfig] = await Promise.all([
        request('/api/overview'),
        request('/api/boards'),
        request('/api/config').catch(() => null)
      ]);
      setDashboard(nextDashboard);
      setBoards(nextBoards);
      if (nextConfig) setConfigData(nextConfig);
      setSyncedAt(new Date());
      if (!silent) notify('Datos sincronizados');
    } catch (error) {
      notify(error.message, true);
    }
  }, []);

  const loadBoard = useCallback(async id => {
    if (!id) return;
    try {
      setSnapshot(await request(`/api/boards/${encodeURIComponent(id)}`));
    } catch (error) {
      notify(error.message, true);
    }
  }, []);

  useEffect(() => {
    loadAll();
    const timer = setInterval(() => !document.hidden && loadAll(), 8000);
    return () => clearInterval(timer);
  }, [loadAll]);

  useEffect(() => {
    if (boards.length && !boards.some(board => board.board_id === currentBoard)) {
      setCurrentBoard(boards[0].board_id);
      localStorage.setItem('cortex-board', boards[0].board_id);
    }
  }, [boards, currentBoard]);

  useEffect(() => {
    history.replaceState(null, '', `#${view}`);
  }, [view]);

  useEffect(() => {
    if (view !== 'board' || !currentBoard) return;
    loadBoard(currentBoard);
    const timer = setInterval(() => !document.hidden && loadBoard(currentBoard), 5000);
    return () => clearInterval(timer);
  }, [view, currentBoard, loadBoard]);

  const openBoard = id => {
    setCurrentBoard(id);
    localStorage.setItem('cortex-board', id);
    setView('board');
  };

  const afterBoard = async board => {
    setBoardModal(false);
    await loadAll();
    openBoard(board.board_id);
    notify('Sesión creada');
  };

  const afterTask = async task => {
    setTaskModal(false);
    setCurrentBoard(task.board_id);
    await loadAll();
    await loadBoard(task.board_id);
    setView('board');
    notify('Tarea creada');
  };

  const sectionTitle = useMemo(() => {
    if (view === 'board') return boards.find(board => board.board_id === currentBoard)?.title || 'TASK BOARD';
    if (view === 'overview') return 'RESUMEN OPERACIONAL';
    if (view === 'sessions') return 'SESIONES Y BOARDS';
    if (view === 'delegations') return 'MONITOR DE DELEGACIÓN';
    if (view === 'activity') return 'HISTORIAL DE EVENTOS';
    if (view === 'settings') return 'ECOSISTEMA Y CONFIGURACIÓN';
    return view.toUpperCase();
  }, [view, boards, currentBoard]);

  return (
    <div class="shell">
      <Sidebar
        boards={boards}
        dashboard={dashboard}
        currentBoard={currentBoard}
        view={view}
        onView={setView}
        onBoard={openBoard}
        onNewBoard={() => setBoardModal(true)}
      />
      <main>
        <header class="global-header">
          <div>
            <span class="breadcrumb">CORTEX / <b>{sectionTitle}</b></span>
            <span class="live"><i></i>LIVE</span>
          </div>
          <div class="header-actions">
            <span>{syncedAt ? `Actualizado ${new Intl.DateTimeFormat('es-CO', { timeStyle: 'short' }).format(syncedAt)}` : 'Sincronizando…'}</span>
            <button class="icon-button" onClick={() => loadAll(false)} aria-label="Actualizar datos" title="Refrescar datos">↻</button>
          </div>
        </header>

        <section class={`view ${view === 'overview' ? 'active' : ''}`}>
          {view === 'overview' && (
            <Overview
              dashboard={dashboard}
              onView={setView}
              onTask={setDetail}
              onNewTask={() => setTaskModal(true)}
              onJob={setJobDetail}
            />
          )}
        </section>

        <section class={`view ${view === 'sessions' ? 'active' : ''}`}>
          {view === 'sessions' && (
            <Sessions sessions={dashboard.sessions} onBoard={openBoard} onNew={() => setBoardModal(true)} />
          )}
        </section>

        <section class={`view ${view === 'delegations' ? 'active' : ''}`}>
          {view === 'delegations' && (
            <Delegations jobs={dashboard.delegations} onOpenJob={setJobDetail} />
          )}
        </section>

        <section class={`view ${view === 'activity' ? 'active' : ''}`}>
          {view === 'activity' && (
            <>
              <PageHeader
                eyebrow="AUDIT STREAM"
                title="Historial de Eventos"
                description="Registro append-only de transiciones de tareas, claims, leases y delegaciones en SQLite."
              />
              <section class="panel">
                <EventList events={dashboard.activity} />
              </section>
            </>
          )}
        </section>

        <section class={`view ${view === 'settings' ? 'active' : ''}`}>
          {view === 'settings' && (
            <SettingsView configData={configData} onRefresh={() => loadAll(false)} />
          )}
        </section>

        <section class={`view ${view === 'board' ? 'active' : ''}`}>
          {view === 'board' && (
            <Board
              snapshot={snapshot}
              activity={dashboard.activity}
              delegations={dashboard.delegations}
              onNewTask={() => setTaskModal(true)}
              onTask={setDetail}
            />
          )}
        </section>

        {notice && <div class={`notice show ${notice.error ? 'error' : ''}`} role="status">{notice.message}</div>}
      </main>

      <BoardForm
        open={boardModal}
        onClose={() => setBoardModal(false)}
        onCreated={afterBoard}
        onError={error => notify(error.message, true)}
      />
      <TaskForm
        open={taskModal}
        onClose={() => setTaskModal(false)}
        onCreated={afterTask}
        onError={error => notify(error.message, true)}
        boards={boards}
        currentBoard={currentBoard}
      />
      <TaskDetail item={detail} onClose={() => setDetail(null)} />
      <JobDetail job={jobDetail} onClose={() => setJobDetail(null)} />
    </div>
  );
}

render(<App />, document.getElementById('app'));
