import { useEffect, useState } from "react";
import { api, FileMeta } from "../api/client";

interface Props {
  files: FileMeta[];
  onDeleted: () => void;
}

function humanSize(bytes: number): string {
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
}

// FileList renders the stored items and lets the user preview or delete them.
export default function FileList({ files, onDeleted }: Props) {
  if (files.length === 0) {
    return <p className="empty">Nothing stored yet. Add a note or upload a file.</p>;
  }
  return (
    <div className="grid">
      {files.map((f) => (
        <FileCard key={f.id} file={f} onDeleted={onDeleted} />
      ))}
    </div>
  );
}

function FileCard({ file, onDeleted }: { file: FileMeta; onDeleted: () => void }) {
  const [noteText, setNoteText] = useState<string>(file.content ?? "");
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (open && file.kind !== "image" && !noteText) {
      api.getNoteText(file.id).then(setNoteText).catch(() => {});
    }
  }, [open, file, noteText]);

  async function remove() {
    await api.deleteFile(file.id);
    onDeleted();
  }

  return (
    <article className={`card kind-${file.kind}`}>
      <header>
        <span className="badge">{file.kind}</span>
        <strong className="name">{file.name}</strong>
      </header>

      <div className="meta">
        {humanSize(file.size)} · {new Date(file.createdAt).toLocaleString()}
      </div>

      {open && (
        <div className="preview">
          {file.kind === "image" ? (
            <img src={api.fileUrl(file.id)} alt={file.name} />
          ) : (
            <pre>{noteText || "(loading...)"}</pre>
          )}
        </div>
      )}

      <footer>
        <button onClick={() => setOpen((v) => !v)}>{open ? "Hide" : "View"}</button>
        <a href={api.fileUrl(file.id)} target="_blank" rel="noreferrer">
          Open
        </a>
        <button className="danger" onClick={remove}>
          Delete
        </button>
      </footer>
    </article>
  );
}
