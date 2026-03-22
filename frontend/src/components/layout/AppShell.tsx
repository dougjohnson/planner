import { Outlet, NavLink, useParams } from "react-router-dom";
import styles from "./AppShell.module.css";

/**
 * Application shell providing:
 * - Desktop: sticky header + sidebar nav + main content
 * - Mobile:  sticky header + main content + fixed bottom tab bar
 *
 * The sidebar shows project-scoped navigation when a project is selected.
 * The bottom tab bar provides primary navigation on touch devices.
 */
export function AppShell() {
  const { projectId } = useParams<{ projectId?: string }>();

  return (
    <div className={styles.shell}>
      <a href="#main-content" className={styles.skipLink}>
        Skip to main content
      </a>

      {/* ── Header ── */}
      <header className={styles.header}>
        <NavLink to="/projects" className={styles.logo}>
          <span className={styles.logoMark} aria-hidden="true" />
          Flywheel Planner
        </NavLink>
        <nav className={styles.headerNav} aria-label="Global">
          <NavLink to="/models" className={headerNavClass}>
            Models
          </NavLink>
          <NavLink to="/prompts" className={headerNavClass}>
            Prompts
          </NavLink>
          <NavLink to="/settings" className={headerNavClass}>
            Settings
          </NavLink>
        </nav>
      </header>

      {/* ── Body: sidebar + main ── */}
      <div className={styles.body}>
        <aside className={styles.sidebar} aria-label="Navigation">
          <nav>
            <ul className={styles.navList}>
              <li>
                <NavLink to="/projects" className={sideNavClass} end>
                  Projects
                </NavLink>
              </li>
              {projectId && <ProjectNav projectId={projectId} />}
            </ul>
          </nav>
        </aside>

        <main id="main-content" className={styles.main}>
          <Outlet />
        </main>
      </div>

      {/* ── Mobile bottom tab bar ── */}
      <nav className={styles.bottomBar} aria-label="Navigation">
        <NavLink to="/projects" className={tabClass} end>
          <TabIcon d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-6 0a1 1 0 001-1v-4a1 1 0 011-1h2a1 1 0 011 1v4a1 1 0 001 1m-6 0h6" />
          <span className={styles.tabLabel}>Projects</span>
        </NavLink>
        {projectId ? (
          <>
            <NavLink to={`/projects/${projectId}`} className={tabClass} end>
              <TabIcon d="M9 19v-6a2 2 0 00-2-2H5a2 2 0 00-2 2v6a2 2 0 002 2h2a2 2 0 002-2zm0 0V9a2 2 0 012-2h2a2 2 0 012 2v10m-6 0a2 2 0 002 2h2a2 2 0 002-2m0 0V5a2 2 0 012-2h2a2 2 0 012 2v14a2 2 0 01-2 2h-2a2 2 0 01-2-2z" />
              <span className={styles.tabLabel}>Dashboard</span>
            </NavLink>
            <NavLink to={`/projects/${projectId}/export`} className={tabClass}>
              <TabIcon d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              <span className={styles.tabLabel}>Export</span>
            </NavLink>
          </>
        ) : null}
        <NavLink to="/models" className={tabClass}>
          <TabIcon d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
          <span className={styles.tabLabel}>Settings</span>
        </NavLink>
      </nav>

      {/* Screen reader live region */}
      <div
        className={styles.liveRegion}
        role="status"
        aria-live="polite"
        aria-atomic="true"
        id="status-announcer"
      />
    </div>
  );
}

/* ── Project-scoped sidebar nav ── */

function ProjectNav({ projectId }: { projectId: string }) {
  const base = `/projects/${projectId}`;
  return (
    <>
      <li className={styles.navSection}>Project</li>
      <li>
        <NavLink to={base} className={sideNavClass} end>
          Dashboard
        </NavLink>
      </li>
      <li>
        <NavLink to={`${base}/foundations`} className={sideNavClass}>
          Foundations
        </NavLink>
      </li>
      <li>
        <NavLink to={`${base}/prompts`} className={sideNavClass}>
          Prompts
        </NavLink>
      </li>
      <li>
        <NavLink to={`${base}/export`} className={sideNavClass}>
          Export
        </NavLink>
      </li>
    </>
  );
}

/* ── Nav class helpers ── */

function sideNavClass({ isActive }: { isActive: boolean }) {
  return isActive ? styles.navLinkActive : styles.navLink;
}

function headerNavClass({ isActive }: { isActive: boolean }) {
  return isActive ? styles.headerLinkActive : styles.headerLink;
}

function tabClass({ isActive }: { isActive: boolean }) {
  return isActive ? styles.tabLinkActive : styles.tabLink;
}

/* ── Inline SVG icon for bottom tabs (Heroicons outline style) ── */

function TabIcon({ d }: { d: string }) {
  return (
    <svg
      className={styles.tabIcon}
      fill="none"
      viewBox="0 0 24 24"
      stroke="currentColor"
      strokeWidth={1.5}
      aria-hidden="true"
    >
      <path strokeLinecap="round" strokeLinejoin="round" d={d} />
    </svg>
  );
}
