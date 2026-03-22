/**
 * UsageSummary — model usage and estimated cost display for the project dashboard.
 * Shows token consumption and estimated costs per provider/model.
 */

interface UsageRecord {
  provider: string;
  model_name: string;
  input_tokens: number;
  output_tokens: number;
  cached_tokens: number;
  estimated_cost_minor: number;
}

interface UsageSummaryProps {
  records: UsageRecord[];
}

export default function UsageSummary({ records }: UsageSummaryProps) {
  if (records.length === 0) {
    return (
      <div className="usage-summary usage-summary--empty">
        <h3>Model Usage</h3>
        <p>No model runs recorded yet.</p>
      </div>
    );
  }

  // Aggregate by provider.
  const byProvider = new Map<
    string,
    { input: number; output: number; cached: number; cost: number; runs: number }
  >();

  for (const r of records) {
    const key = r.provider;
    const existing = byProvider.get(key) || {
      input: 0,
      output: 0,
      cached: 0,
      cost: 0,
      runs: 0,
    };
    existing.input += r.input_tokens;
    existing.output += r.output_tokens;
    existing.cached += r.cached_tokens;
    existing.cost += r.estimated_cost_minor;
    existing.runs += 1;
    byProvider.set(key, existing);
  }

  const totalInput = records.reduce((s, r) => s + r.input_tokens, 0);
  const totalOutput = records.reduce((s, r) => s + r.output_tokens, 0);
  const totalCost = records.reduce((s, r) => s + r.estimated_cost_minor, 0);

  return (
    <div className="usage-summary">
      <h3>Model Usage</h3>

      {/* Total summary */}
      <div className="usage-total">
        <span className="usage-stat">
          <strong>{formatTokens(totalInput)}</strong> input tokens
        </span>
        <span className="usage-stat">
          <strong>{formatTokens(totalOutput)}</strong> output tokens
        </span>
        <span className="usage-stat">
          <strong>{formatCost(totalCost)}</strong> estimated cost
        </span>
        <span className="usage-stat">
          <strong>{records.length}</strong> runs
        </span>
      </div>

      {/* Per-provider breakdown */}
      <table className="usage-table" aria-label="Usage by provider">
        <thead>
          <tr>
            <th>Provider</th>
            <th>Runs</th>
            <th>Input</th>
            <th>Output</th>
            <th>Est. Cost</th>
          </tr>
        </thead>
        <tbody>
          {Array.from(byProvider.entries()).map(([provider, stats]) => (
            <tr key={provider}>
              <td>{provider}</td>
              <td>{stats.runs}</td>
              <td>{formatTokens(stats.input)}</td>
              <td>{formatTokens(stats.output)}</td>
              <td>{formatCost(stats.cost)}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}K`;
  return String(n);
}

function formatCost(minorUnits: number): string {
  // Minor units are 1/100 of a cent.
  const cents = minorUnits / 100;
  if (cents >= 100) return `$${(cents / 100).toFixed(2)}`;
  return `${cents.toFixed(1)}¢`;
}
