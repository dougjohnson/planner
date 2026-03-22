/**
 * Settings page — global app configuration and operational defaults.
 */

import { useState, useCallback } from "react";

interface AppSettings {
  data_dir: string;
  listen_addr: string;
  default_loop_count: number;
  default_export_mode: string;
  worker_pool_concurrency: number;
}

const DEFAULT_SETTINGS: AppSettings = {
  data_dir: "~/.flywheel-planner",
  listen_addr: "127.0.0.1:7432",
  default_loop_count: 4,
  default_export_mode: "canonical_only",
  worker_pool_concurrency: 4,
};

export default function Settings() {
  const [settings, setSettings] = useState<AppSettings>(DEFAULT_SETTINGS);
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState(false);

  const handleSave = useCallback(async () => {
    setSaving(true);
    setSaved(false);
    try {
      await fetch("/api/settings", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(settings),
      });
      setSaved(true);
    } catch {
      // Best effort.
    } finally {
      setSaving(false);
    }
  }, [settings]);

  return (
    <div className="settings-page">
      <h1>Settings</h1>

      <section className="settings-section">
        <h2>Application</h2>
        <div className="setting-field">
          <label htmlFor="data-dir">Data Directory</label>
          <input
            id="data-dir"
            type="text"
            value={settings.data_dir}
            readOnly
            className="setting-input setting-input--readonly"
          />
          <span className="setting-help">Read-only — set via FLYWHEEL_DATA_DIR env var</span>
        </div>

        <div className="setting-field">
          <label htmlFor="listen-addr">Listen Address</label>
          <input
            id="listen-addr"
            type="text"
            value={settings.listen_addr}
            readOnly
            className="setting-input setting-input--readonly"
          />
          <span className="setting-help">Read-only — set via FLYWHEEL_LISTEN_ADDR env var</span>
        </div>
      </section>

      <section className="settings-section">
        <h2>Workflow Defaults</h2>

        <div className="setting-field">
          <label htmlFor="loop-count">Default Loop Count</label>
          <input
            id="loop-count"
            type="number"
            min={1}
            max={10}
            value={settings.default_loop_count}
            onChange={(e) =>
              setSettings((s) => ({
                ...s,
                default_loop_count: parseInt(e.target.value, 10) || 4,
              }))
            }
            className="setting-input"
          />
          <span className="setting-help">
            Number of review iterations per loop (PRD and Plan). Higher = more
            refinement but more API cost.
          </span>
        </div>

        <div className="setting-field">
          <label htmlFor="export-mode">Default Export Mode</label>
          <select
            id="export-mode"
            value={settings.default_export_mode}
            onChange={(e) =>
              setSettings((s) => ({
                ...s,
                default_export_mode: e.target.value,
              }))
            }
            className="setting-input"
          >
            <option value="canonical_only">Canonical Only</option>
            <option value="include_intermediates">Include Intermediates</option>
            <option value="include_all">Include All (with raw outputs)</option>
          </select>
        </div>

        <div className="setting-field">
          <label htmlFor="concurrency">Worker Pool Concurrency</label>
          <input
            id="concurrency"
            type="number"
            min={1}
            max={16}
            value={settings.worker_pool_concurrency}
            onChange={(e) =>
              setSettings((s) => ({
                ...s,
                worker_pool_concurrency: parseInt(e.target.value, 10) || 4,
              }))
            }
            className="setting-input"
          />
          <span className="setting-help">
            Max simultaneous model executions.
          </span>
        </div>
      </section>

      <section className="settings-section">
        <h2>Model Credentials</h2>
        <p>
          <a href="/models">Manage model configurations and API keys</a>
        </p>
      </section>

      <div className="settings-actions">
        <button
          onClick={handleSave}
          disabled={saving}
          className="save-button"
        >
          {saving ? "Saving..." : "Save Settings"}
        </button>
        {saved && <span className="save-confirmation">Settings saved</span>}
      </div>
    </div>
  );
}
