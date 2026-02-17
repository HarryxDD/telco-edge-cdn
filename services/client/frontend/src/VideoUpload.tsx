import React, { useState } from "react";

interface VideoUploadProps {
  onUploadComplete: () => void;
}

function VideoUpload({ onUploadComplete }: VideoUploadProps) {
  const [uploading, setUploading] = useState(false);
  const [progress, setProgress] = useState(0);
  const [message, setMessage] = useState("");

  const handleFileChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;

    // Validate file type
    if (!file.type.startsWith("video/")) {
      setMessage("Please select a video file (MP4, MOV, AVI, etc.)");
      return;
    }

    // Validate file size (max 500MB)
    const maxSize = 500 * 1024 * 1024;
    if (file.size > maxSize) {
      setMessage("File too large! Maximum size is 500MB");
      return;
    }

    setUploading(true);
    setProgress(0);
    setMessage("Uploading and processing video...");

    const formData = new FormData();
    formData.append("file", file);

    try {
      const xhr = new XMLHttpRequest();

      // Track upload progress
      xhr.upload.addEventListener("progress", (e) => {
        if (e.lengthComputable) {
          const percentComplete = (e.loaded / e.total) * 100;
          setProgress(percentComplete);
        }
      });

      xhr.addEventListener("load", () => {
        if (xhr.status === 202) {
          const response = JSON.parse(xhr.responseText);
          setMessage(
            `Upload complete! Processing video: ${response.title}`
          );
          setProgress(100);
          setTimeout(() => {
            onUploadComplete();
            setUploading(false);
            setMessage("");
            setProgress(0);
          }, 2000);
        } else {
          setMessage(`Upload failed: ${xhr.statusText}`);
          setUploading(false);
        }
      });

      xhr.addEventListener("error", () => {
        setMessage("Upload failed. Please try again.");
        setUploading(false);
      });

      // Upload via load balancer (which proxies to origin)
      xhr.open("POST", "/api/upload");
      xhr.send(formData);
    } catch (error) {
      console.error("Upload error:", error);
      setMessage("Upload failed. Please try again.");
      setUploading(false);
    }
  };

  return (
    <div
      style={{
        padding: "16px",
        borderBottom: "1px solid #e5e7eb",
        backgroundColor: "#f9fafb",
      }}
    >
      <div style={{ marginBottom: "8px" }}>
        <label
          htmlFor="video-upload"
          style={{
            display: "inline-block",
            padding: "10px 20px",
            backgroundColor: uploading ? "#9ca3af" : "#3b82f6",
            color: "white",
            borderRadius: "6px",
            cursor: uploading ? "not-allowed" : "pointer",
            fontWeight: 500,
            transition: "background-color 0.2s",
          }}
        >
          {uploading ? "Processing..." : "Upload Video"}
        </label>
        <input
          id="video-upload"
          type="file"
          accept="video/*"
          onChange={handleFileChange}
          disabled={uploading}
          style={{ display: "none" }}
        />
      </div>

      {uploading && (
        <div style={{ marginTop: "8px" }}>
          <div
            style={{
              width: "100%",
              height: "6px",
              backgroundColor: "#e5e7eb",
              borderRadius: "3px",
              overflow: "hidden",
            }}
          >
            <div
              style={{
                width: `${progress}%`,
                height: "100%",
                backgroundColor: "#3b82f6",
                transition: "width 0.3s ease",
              }}
            />
          </div>
          <div style={{ fontSize: "12px", color: "#6b7280", marginTop: "4px" }}>
            {Math.round(progress)}%
          </div>
        </div>
      )}

      {message && (
        <div
          style={{
            marginTop: "8px",
            padding: "8px 12px",
            backgroundColor: message.startsWith("✅")
              ? "#d1fae5"
              : message.startsWith("❌")
              ? "#fee2e2"
              : "#dbeafe",
            borderRadius: "4px",
            fontSize: "14px",
            color: message.startsWith("✅")
              ? "#065f46"
              : message.startsWith("❌")
              ? "#991b1b"
              : "#1e40af",
          }}
        >
          {message}
        </div>
      )}
    </div>
  );
}

export default VideoUpload;
