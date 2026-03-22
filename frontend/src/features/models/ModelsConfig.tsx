/**
 * ModelsConfig — model provider management at /models route.
 * Configure providers, enter credentials, trigger validation, toggle enabled.
 */

import { useState, useCallback } from "react";

interface ModelConfig {
  id: string;
  provider: string;
  model_name: string;
  validation_status: string;
  enabled_global: boolean;
}

interface ProviderStatus {
  provider: string;
  displayName: string;
  envVar: string;
  models: ModelConfig[];
  hasCredential: boolean;
  validationStatus: string;
}

const PROVIDERS: ProviderStatus[] = [
  {
    provider: "openai",
    displayName: "OpenAI",
    envVar: "FLYWHEEL_OPENAI_API_KEY",
    models: [],
    hasCredential: false,
    validationStatus: "unchecked",
  },
  {
    provider: "anthropic",
    displayName: "Anthropic",
    envVar: "FLYWHEEL_ANTHROPIC_API_KEY",
    models: [],
    hasCredential: false,
    validationStatus: "unchecked",
  },
];

export default function ModelsConfig() {
  const [providers] = useState<ProviderStatus[]>(PROVIDERS);
  const [credentials, setCredentials] = useState<Record<string, string>>({});
  const [validating, setValidating] = useState<string | null>(null);
  const [validationResults, setValidationResults] = useState<
    Record<string, { status: string; message: string }>
  >({});
  const [saving, setSaving] = useState<string | null>(null);

  const handleSaveCredential = useCallback(
    async (provider: string) => {
      const key = credentials[provider];
      if (!key?.trim()) return;

      setSaving(provider);
      try {
        await fetch(`/api/models/credentials/${encodeURIComponent(provider)}`, {
          method: "PUT",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ api_key: key }),
        });
        setCredentials((c) => ({ ...c, [provider]: "" }));
      } catch {
        // Best effort.
      } finally {
        setSaving(null);
      }
    },
    [credentials],
  );

  const handleValidate = useCallback(async (provider: string) => {
    setValidating(provider);
    try {
      const response = await fetch(
        `/api/models/validate/${encodeURIComponent(provider)}`,
        { method: "POST" },
      );
      const result = await response.json();
      setValidationResults((r) => ({
        ...r,
        [provider]: { status: result.status, message: result.message },
      }));
    } catch {
      setValidationResults((r) => ({
        ...r,
        [provider]: { status: "error", message: "Validation request failed" },
      }));
    } finally {
      setValidating(null);
    }
  }, []);

  return (
    <div className="models-config">
      <h1>Model Configuration</h1>
      <p>
        Configure LLM providers and API credentials. At least one GPT-family and
        one Opus-family model must be configured.
      </p>

      {providers.map((p) => (
        <section key={p.provider} className="provider-card">
          <div className="provider-header">
            <h2>{p.displayName}</h2>
            <span
              className={`validation-badge validation-badge--${
                validationResults[p.provider]?.status || p.validationStatus
              }`}
            >
              {validationResults[p.provider]?.status || p.validationStatus}
            </span>
          </div>

          {/* Credential entry */}
          <div className="credential-section">
            <label htmlFor={`key-${p.provider}`}>API Key</label>
            <div className="credential-input-row">
              <input
                id={`key-${p.provider}`}
                type="password"
                value={credentials[p.provider] || ""}
                onChange={(e) =>
                  setCredentials((c) => ({
                    ...c,
                    [p.provider]: e.target.value,
                  }))
                }
                placeholder={`Enter ${p.displayName} API key`}
                className="credential-input"
                autoComplete="off"
              />
              <button
                onClick={() => handleSaveCredential(p.provider)}
                disabled={
                  saving === p.provider || !credentials[p.provider]?.trim()
                }
                className="credential-save"
              >
                {saving === p.provider ? "Saving..." : "Save"}
              </button>
            </div>
            <span className="credential-hint">
              Or set via env var: {p.envVar}
            </span>
          </div>

          {/* Validation */}
          <div className="validation-section">
            <button
              onClick={() => handleValidate(p.provider)}
              disabled={validating === p.provider}
              className="validate-button"
            >
              {validating === p.provider
                ? "Validating..."
                : "Validate Credentials"}
            </button>
            {validationResults[p.provider] && (
              <span
                className={`validation-result validation-result--${validationResults[p.provider].status}`}
              >
                {validationResults[p.provider].message}
              </span>
            )}
          </div>
        </section>
      ))}
    </div>
  );
}
