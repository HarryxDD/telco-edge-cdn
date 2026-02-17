import React, { useEffect, useState } from "react";
import "./App.css";
import VideoList, { VideoMeta } from "./VideoList";
import VideoPlayer from "./VideoPlayer";
import VideoUpload from "./VideoUpload";

function App() {
  const [videos, setVideos] = useState<VideoMeta[]>([]);
  const [selectedTitle, setSelectedTitle] = useState<string | undefined>();

  const fetchVideos = async () => {
    try {
      const res = await fetch("/api/videos");
      if (!res.ok) return;
      const data = await res.json();
      setVideos(data);
    } catch (e) {
      console.error("Failed to fetch videos", e);
    }
  };

  useEffect(() => {
    fetchVideos();
  }, []);

  return (
    <div style={{ display: "flex", height: "100vh" }}>
      <div style={{ flex: 1, borderRight: "1px solid #e5e7eb" }}>
        <div
          style={{
            padding: 16,
            display: "flex",
            justifyContent: "space-between",
            alignItems: "center",
          }}
        >
          <h1>CDN Streaming App</h1>
        </div>
        <VideoUpload onUploadComplete={fetchVideos} />
        <VideoList
          videos={videos}
          onSelect={(video) => setSelectedTitle(video.title)}
          selectedTitle={selectedTitle}
        />
      </div>
      <div style={{ flex: 1 }}>
        <VideoPlayer title={selectedTitle} />
      </div>
    </div>
  );
}

export default App;
