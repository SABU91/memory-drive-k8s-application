import { useState } from "react";
import { api } from "../api/client";

interface Props {
  onUploaded: () => void;
}

// UploadPanel lets the user create a note or upload a text/image file.
export default function UploadPanel({ onUploaded }: Props) {
  const [tab, setTab] = useState<"note" | "file">("note");
  const [noteName, setNoteName] = useState("");
  const [noteBody, setNoteBody] = useState("");
  const [file, setFile] = useState<File | null>(null);
  const [fileName, setFileName] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  async function submit() {
    setBusy(true);
    setError("");
    try {
      if (tab === "note") {
        if (!noteBody.trim()) throw new Error("Note content is empty");
        await api.uploadNote(noteName.trim(), noteBody);
        setNoteName("");
        setNoteBody("");
      } else {
        if (!file) throw new Error("Choose a file first");
        await api.uploadFile(file, fileName.trim());
        setFile(null);
        setFileName("");
      }
      onUploaded();
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
    } finally {
      setBusy(false);
    }
  }

  return (
    <section className="panel">
      <h2>Add to your drive</h2>

      <div className="tabs">
        <button className={tab === "note" ? "active" : ""} onClick={() => setTab("note")}>
          Note
        </button>
        <button className={tab === "file" ? "active" : ""} onClick={() => setTab("file")}>
          File / Image
        </button>
      </div>

      {tab === "note" ? (
        <div className="form">
          <input
            placeholder="Title (optional)"
            value={noteName}
            onChange={(e) => setNoteName(e.target.value)}
          />
          <textarea
            placeholder="Write a note..."
            rows={5}
            value={noteBody}
            onChange={(e) => setNoteBody(e.target.value)}
          />
        </div>
      ) : (
        <div className="form">
          <input
            placeholder="Name (optional)"
            value={fileName}
            onChange={(e) => setFileName(e.target.value)}
          />
          <input
            type="file"
            accept="text/*,image/*"
            onChange={(e) => setFile(e.target.files?.[0] ?? null)}
          />
        </div>
      )}

      {error && <p className="error">{error}</p>}

      <button className="primary" onClick={submit} disabled={busy}>
        {busy ? "Uploading..." : "Upload"}
      </button>
    </section>
  );
}
