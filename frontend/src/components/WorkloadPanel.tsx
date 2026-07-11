import { useState } from "react";
import { api, Stats } from "../api/client";

interface Props {
  stats: Stats | null;
  onChanged: () => void;
}

function mb(bytes: number): string {
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

// WorkloadPanel shows live resource stats and lets you trigger memory/CPU load
// so you can watch utilisation change in Grafana.
export default function WorkloadPanel({ stats, onChanged }: Props) {
  const [memMB, setMemMB] = useState(100);
  const [holdSec, setHoldSec] = useState(0);
  const [loadSec, setLoadSec] = useState(15);
  const [workers, setWorkers] = useState(2);
  const [msg, setMsg] = useState("");

  async function allocate() {
    await api.simulateMemory(memMB, holdSec);
    setMsg(`Allocated ${memMB} MB`);
    onChanged();
  }

  async function runLoad() {
    await api.simulateLoad(loadSec, workers);
    setMsg(`Running CPU load: ${workers} workers for ${loadSec}s`);
  }

  return (
    <section className="panel">
      <h2>Resource playground</h2>

      {stats && (
        <div className="stats">
          <div><span>Files</span><b>{stats.files}</b></div>
          <div><span>Cache</span><b>{stats.cacheMB} MB</b></div>
          <div><span>Allocated</span><b>{stats.allocatedMB} MB</b></div>
          <div><span>Heap</span><b>{mb(stats.heapAllocBytes)}</b></div>
          <div><span>Sys</span><b>{mb(stats.sysBytes)}</b></div>
          <div><span>Goroutines</span><b>{stats.goroutines}</b></div>
          <div><span>Workers</span><b>{stats.config.backgroundWorkers ? stats.workerCount : "off"}</b></div>
          <div><span>Uptime</span><b>{stats.uptimeSeconds}s</b></div>
        </div>
      )}

      <div className="controls">
        <fieldset>
          <legend>Allocate memory</legend>
          <label>MB <input type="number" min={1} value={memMB} onChange={(e) => setMemMB(+e.target.value)} /></label>
          <label>Hold (s, 0 = forever) <input type="number" min={0} value={holdSec} onChange={(e) => setHoldSec(+e.target.value)} /></label>
          <button onClick={allocate}>Allocate</button>
        </fieldset>

        <fieldset>
          <legend>CPU load burst</legend>
          <label>Seconds <input type="number" min={1} value={loadSec} onChange={(e) => setLoadSec(+e.target.value)} /></label>
          <label>Workers <input type="number" min={1} value={workers} onChange={(e) => setWorkers(+e.target.value)} /></label>
          <button onClick={runLoad}>Run load</button>
        </fieldset>
      </div>

      {msg && <p className="ok">{msg}</p>}
    </section>
  );
}
