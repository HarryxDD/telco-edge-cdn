import React, { useState } from "react";

interface VideoUploadProps {
  onUploadComplete: () => void;
}

function VideoUpload({ onUploadComplete }: VideoUploadProps) {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");
  const [messageType, setMessageType] = useState<"info" | "success" | "error">("info");

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    if (!file.type.startsWith("video/")) {
      setMessage("Invalid file type. Select a video file.");
      setMessageType("error");
      return;
    }

    if (file.size > 500 * 1024 * 1024) {
      setMessage("File exceeds 500MB limit.");
      setMessageType("error");
      return;
    }

    setUploading(true);
    setProgress(0);
    setMessage("Transferring to edge origin...");
    setMessageType("info");

    const formData = new FormData();
    formData.append("file", file);

    const xhr = new XMLHttpRequest();

    xhr.upload.addEventListener("progress", (e) => {
      if (e.lengthComputable) {
        setProgress((e.loaded / e.total) * 100);
      }
    });

    xhr.addEventListener("load", () => {
      if (xhr.status === 202) {
        const response = JSON.parse(xhr.responseText);
        setMessage(`Encoding: ${response.title}`);
        setMessageType("success");
        setProgress(100);
        setTimeout(() => {
          onUploadComplete();
          setUploading(false);
          setMessage("");
          setProgress(0);
        }, 2500);
      } else {
        setMessage("Transfer failed. Retry.");
        setMessageType("error");
        setUploading(false);
      }
    });

    xhr.addEventListener("error", () => {
      setMessage("Connection error.");
      setMessageType("error");
      setUploading(false);
    });

    xhr.open("POST", "/api/upload");
    xhr.send(formData);
  };

  return (
    <div style={styles.container}>
      <label style={{
        ...styles.uploadBtn,
        ...(uploading ? styles.uploadBtnDisabled : {}),
      }}>
        <input
          type="file"
          accept="video/*"
          onChange={handleFileChange}
          disabled={uploading}
          style={{ display: "none" }}
        />
        <span style={styles.btnIcon}>{uploading ? "⟳" : "↑"}</span>
        <span>{uploading ? "UPLOADING..." : "SELECT FILE"}</span>
      </label>

      {uploading && (
        <div style={styles.progressContainer}>
          <div style={styles.progressTrack}>
            <div style={{ ...styles.progressBar, width: `${progress}%` }} />
            <div style={{ ...styles.progressGlow, width: `${progress}%` }} />
          </div>
          <div style={styles.progressText}>
            <span>{Math.round(progress)}%</span>
            <span style={styles.progressBytes}>
              {file_size_display(progress)}
            </span>
          </div>
        </div>
      )}

      {message && (
        <div style={{
          ...styles.message,
          ...(messageType === "success" ? styles.messageSuccess : {}),
          ...(messageType === "error" ? styles.messageError : {}),
        }}>
          <span style={styles.messageIcon}>
            {messageType === "success" ? "✓" : messageType === "error" ? "✕" : "·"}
          </span>
          {message}
        </div>
      )}
    </div>
  );
}

function file_size_display(progress: number): string {
  if (progress < 30) return "BUFFERING";
  if (progress < 70) return "STREAMING";
  if (progress < 100) return "FINALIZING";
  return "COMPLETE";
}

const styles: Record<string, React.CSSProperties> = {
  container: {
    padding: "16px 20px",
    display: "flex",
    flexDirection: "column",
    gap: "10px",
  },
  uploadBtn: {
    display: "flex",
    alignItems: "center",
    gap: "10px",
    padding: "10px 16px",
    background: "transparent",
    border: "1px solid rgba(0,255,170,0.4)",
    borderRadius: "2px",
    color: "#00ffaa",
    fontSize: "11px",
    letterSpacing: "2px",
    cursor: "pointer",
    transition: "all 0.2s",
    fontFamily: "inherit",
  },
  uploadBtnDisabled: {
    opacity: 0.5,
    cursor: "not-allowed",
    borderColor: "rgba(0,255,170,0.2)",
  },
  btnIcon: {
    fontSize: "14px",
  },
  progressContainer: {
    display: "flex",
    flexDirection: "column",
    gap: "4px",
  },
  progressTrack: {
    height: "2px",
    background: "rgba(0,255,170,0.1)",
    borderRadius: "1px",
    position: "relative",
    overflow: "hidden",
  },
  progressBar: {
    position: "absolute",
    top: 0,
    left: 0,
    height: "100%",
    background: "#00ffaa",
    transition: "width 0.3s ease",
  },
  progressGlow: {
    position: "absolute",
    top: 0,
    left: 0,
    height: "100%",
    background: "rgba(0,255,170,0.3)",
    filter: "blur(4px)",
    transition: "width 0.3s ease",
  },
  progressText: {
    display: "flex",
    justifyContent: "space-between",
    fontSize: "10px",
    color: "rgba(0,255,170,0.6)",
    letterSpacing: "1px",
  },
  progressBytes: {
    letterSpacing: "2px",
  },
  message: {
    display: "flex",
    alignItems: "center",
    gap: "8px",
    padding: "8px 10px",
    background: "rgba(0,200,255,0.05)",
    border: "1px solid rgba(0,200,255,0.2)",
    borderRadius: "2px",
    fontSize: "10px",
    letterSpacing: "1px",
    color: "rgba(0,200,255,0.8)",
  },
  messageSuccess: {
    background: "rgba(0,255,170,0.05)",
    border: "1px solid rgba(0,255,170,0.2)",
    color: "rgba(0,255,170,0.8)",
  },
  messageError: {
    background: "rgba(255,60,60,0.05)",
    border: "1px solid rgba(255,60,60,0.2)",
    color: "rgba(255,60,60,0.8)",
  },
  messageIcon: {
    fontSize: "12px",
    fontWeight: "bold",
  },
};

export default VideoUpload;