import { useCallback, useEffect, useState } from "react";
import { api, FileMeta, Stats } from "./api/client";
import UploadPanel from "./components/UploadPanel";
import FileList from "./components/FileList";
import WorkloadPanel from "./components/WorkloadPanel";

export default function App() {
  const [files, setFiles] = useState<FileMeta[]>([]);
  const [search, setSearch] = useState("");
  const [stats, setStats] = useState<Stats | null>(null);
  const [error, setError] = useState("");

  const refreshFiles = useCallback(async () => {
    try {
      setFiles(await api.listFiles(search));
      setError("");
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    }
  }, [search]);

  const refreshStats = useCallback(async () => {
    try {
      setStats(await api.stats());
    } catch {
      /* stats are best-effort */
    }
  }, []);

  // Reload the file list whenever the search term changes (debounced).
  useEffect(() => {
    const t = setTimeout(refreshFiles, 250);
    return () => clearTimeout(t);
  }, [refreshFiles]);

  // Poll stats every 3 seconds so the resource playground stays live.
  useEffect(() => {
    refreshStats();
    const t = setInterval(refreshStats, 3000);
    return () => clearInterval(t);
  }, [refreshStats]);

  return (
    <div className="app">
      <header className="topbar">
        <h1>🧠 Memory Drive</h1>
        <p>Store notes, text files and images. Watch resources move.</p>
      </header>

      <main className="layout">
        <div className="left">
          <UploadPanel onUploaded={refreshFiles} />
          <WorkloadPanel stats={stats} onChanged={refreshStats} />
        </div>

        <div className="right">
          <div className="searchbar">
            <input
              placeholder="Search notes and files..."
              value={search}
              onChange={(e) => setSearch(e.target.value)}
            />
            <button onClick={refreshFiles}>Refresh</button>
          </div>
          {error && <p className="error">{error}</p>}
          <FileList files={files} onDeleted={refreshFiles} />
        </div>
      </main>
    </div>
  );
}
