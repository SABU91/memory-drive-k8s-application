// API client for the Memory Drive backend.
//
// The base URL is configurable via the VITE_API_BASE environment variable.
// It defaults to "" (same origin), which works both behind the Ingress in
// Kubernetes and behind the Vite dev proxy locally.

const BASE = import.meta.env.VITE_API_BASE ?? "";

export type Kind = "note" | "text" | "image";

export interface FileMeta {
  id: string;
  name: string;
  kind: Kind;
  contentType: string;
  size: number;
  content?: string;
  createdAt: string;
}

export interface Stats {
  uptimeSeconds: number;
  files: number;
  totalFileBytes: number;
  cacheMB: number;
  allocatedMB: number;
  workerCount: number;
  goroutines: number;
  heapAllocBytes: number;
  heapInuseBytes: number;
  sysBytes: number;
  numGC: number;
  config: {
    memoryCacheEnabled: boolean;
    backgroundWorkers: boolean;
    maxUploadMB: number;
  };
}

async function handle<T>(res: Response): Promise<T> {
  if (!res.ok) {
    let message = res.statusText;
    try {
      const body = await res.json();
      if (body?.error) message = body.error;
    } catch {
      /* ignore non-JSON error bodies */
    }
    throw new Error(message);
  }
  return res.json() as Promise<T>;
}

export const api = {
  fileUrl(id: string): string {
    return `${BASE}/files/${id}`;
  },

  async listFiles(search: string): Promise<FileMeta[]> {
    const q = search ? `?search=${encodeURIComponent(search)}` : "";
    const data = await handle<{ files: FileMeta[] }>(await fetch(`${BASE}/files${q}`));
    return data.files;
  },

  async uploadNote(name: string, content: string): Promise<FileMeta> {
    const form = new FormData();
    form.append("name", name);
    form.append("content", content);
    return handle<FileMeta>(await fetch(`${BASE}/upload`, { method: "POST", body: form }));
  },

  async uploadFile(file: File, name: string): Promise<FileMeta> {
    const form = new FormData();
    if (name) form.append("name", name);
    form.append("file", file);
    return handle<FileMeta>(await fetch(`${BASE}/upload`, { method: "POST", body: form }));
  },

  async deleteFile(id: string): Promise<void> {
    const res = await fetch(`${BASE}/files/${id}`, { method: "DELETE" });
    if (!res.ok) throw new Error(`delete failed: ${res.statusText}`);
  },

  async getNoteText(id: string): Promise<string> {
    const res = await fetch(`${BASE}/files/${id}`);
    if (!res.ok) throw new Error(`fetch failed: ${res.statusText}`);
    return res.text();
  },

  async stats(): Promise<Stats> {
    return handle<Stats>(await fetch(`${BASE}/stats`));
  },

  async simulateMemory(megabytes: number, holdSeconds: number): Promise<unknown> {
    return handle(
      await fetch(`${BASE}/simulate/memory`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ megabytes, holdSeconds }),
      })
    );
  },

  async simulateLoad(durationSeconds: number, workers: number): Promise<unknown> {
    return handle(
      await fetch(`${BASE}/simulate/load`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ durationSeconds, workers, async: true }),
      })
    );
  },
};
