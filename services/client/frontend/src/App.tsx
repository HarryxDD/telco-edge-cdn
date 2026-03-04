import React, { useEffect, useState } from "react";
import VideoList, { VideoMeta } from "./VideoList";
import VideoPlayer from "./VideoPlayer";
import VideoUpload from "./VideoUpload";

function App() {
  const [videos, setVideos] = useState<VideoMeta[]>([]);
  const [selectedTitle, setSelectedTitle] = useState<string | undefined>();
  const [time, setTime] = useState(new Date());

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
    const tick = setInterval(() => setTime(new Date()), 1000);
    return () => clearInterval(tick);
  }, []);

  return (
    <div style={styles.root}>
      {/* Background grid */}
      <div style={styles.grid} />

      {/* Header */}
      <header style={styles.header}>
        <div style={styles.headerLeft}>
          <div style={styles.logo}>
            <span style={styles.logoIcon}>◈</span>
            <span style={styles.logoText}>TELCO</span>
            <span style={styles.logoSub}>EDGE CDN</span>
          </div>
          <div style={styles.statusBar}>
            <span style={styles.statusDot} />
            <span style={styles.statusText}>MEC OULU · LIVE</span>
          </div>
        </div>
        <div style={styles.headerRight}>
          <div style={styles.clock}>
            {time.toLocaleTimeString("en-GB", { hour12: false })}
          </div>
          <div style={styles.nodeInfo}>
            <span style={styles.nodeLabel}>NODES</span>
            <span style={styles.nodeValue}>3/3</span>
          </div>
        </div>
      </header>

      {/* Main layout */}
      <main style={styles.main}>
        {/* Sidebar */}
        <aside style={styles.sidebar}>
          <div style={styles.sidebarSection}>
            <div style={styles.sectionHeader}>
              <span style={styles.sectionIcon}>↑</span>
              UPLOAD
            </div>
            <VideoUpload onUploadComplete={fetchVideos} />
          </div>

          <div style={styles.sidebarSection}>
            <div style={styles.sectionHeader}>
              <span style={styles.sectionIcon}>▤</span>
              LIBRARY
              <span style={styles.badge}>{videos.length}</span>
            </div>
            <VideoList
              videos={videos}
              onSelect={(v) => setSelectedTitle(v.title)}
              selectedTitle={selectedTitle}
            />
          </div>
        </aside>

        {/* Player */}
        <section style={styles.playerSection}>
          <VideoPlayer title={selectedTitle} />
        </section>
      </main>
    </div>
  );
}

const styles: Record<string, React.CSSProperties> = {
  root: {
    minHeight: "100vh",
    background: "#0d1117",
    color: "#e2e8f0",
    fontFamily: "'IBM Plex Mono', 'Courier New', monospace",
    position: "relative",
    overflow: "hidden",
    display: "flex",
    flexDirection: "column",
  },
  grid: {
    position: "fixed",
    inset: 0,
    backgroundImage: `
      linear-gradient(rgba(0,255,170,0.03) 1px, transparent 1px),
      linear-gradient(90deg, rgba(0,255,170,0.03) 1px, transparent 1px)
    `,
    backgroundSize: "40px 40px",
    pointerEvents: "none",
    zIndex: 0,
  },
  header: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "16px 32px",
    borderBottom: "1px solid rgba(0,255,170,0.15)",
    background: "rgba(8,12,16,0.9)",
    backdropFilter: "blur(10px)",
    position: "relative",
    zIndex: 10,
  },
  headerLeft: {
    display: "flex",
    alignItems: "center",
    gap: "24px",
  },
  logo: {
    display: "flex",
    alignItems: "baseline",
    gap: "8px",
  },
  logoIcon: {
    fontSize: "20px",
    color: "#00ffaa",
  },
  logoText: {
    fontSize: "18px",
    fontWeight: "700",
    letterSpacing: "4px",
    color: "#00ffaa",
  },
  logoSub: {
    fontSize: "11px",
    letterSpacing: "3px",
    color: "rgba(0,255,170,0.5)",
  },
  statusBar: {
    display: "flex",
    alignItems: "center",
    gap: "6px",
    padding: "4px 12px",
    border: "1px solid rgba(0,255,170,0.2)",
    borderRadius: "2px",
  },
  statusDot: {
    width: "6px",
    height: "6px",
    borderRadius: "50%",
    background: "#00ffaa",
    boxShadow: "0 0 6px #00ffaa",
    animation: "pulse 2s infinite",
  },
  statusText: {
    fontSize: "10px",
    letterSpacing: "2px",
    color: "rgba(0,255,170,0.7)",
  },
  headerRight: {
    display: "flex",
    alignItems: "center",
    gap: "24px",
  },
  clock: {
    fontSize: "22px",
    fontWeight: "300",
    letterSpacing: "2px",
    color: "rgba(226,232,240,0.6)",
    fontVariantNumeric: "tabular-nums",
  },
  nodeInfo: {
    display: "flex",
    flexDirection: "column",
    alignItems: "flex-end",
    gap: "2px",
  },
  nodeLabel: {
    fontSize: "9px",
    letterSpacing: "2px",
    color: "rgba(226,232,240,0.4)",
  },
  nodeValue: {
    fontSize: "16px",
    color: "#00ffaa",
    fontWeight: "600",
  },
  main: {
    display: "flex",
    flex: 1,
    position: "relative",
    zIndex: 1,
    overflow: "hidden",
  },
  sidebar: {
    width: "360px",
    flexShrink: 0,
    borderRight: "1px solid rgba(0,255,170,0.1)",
    display: "flex",
    flexDirection: "column",
    overflow: "hidden",
    background: "rgba(8,12,16,0.7)",
  },
  sidebarSection: {
    borderBottom: "1px solid rgba(0,255,170,0.08)",
  },
  sectionHeader: {
    display: "flex",
    alignItems: "center",
    gap: "8px",
    padding: "12px 20px",
    fontSize: "10px",
    letterSpacing: "3px",
    color: "rgba(0,255,170,0.6)",
    borderBottom: "1px solid rgba(0,255,170,0.08)",
  },
  sectionIcon: {
    fontSize: "12px",
    color: "#00ffaa",
  },
  badge: {
    marginLeft: "auto",
    background: "rgba(0,255,170,0.1)",
    border: "1px solid rgba(0,255,170,0.3)",
    borderRadius: "2px",
    padding: "1px 6px",
    fontSize: "10px",
    color: "#00ffaa",
  },
  playerSection: {
    flex: 1,
    display: "flex",
    flexDirection: "column",
    overflow: "hidden",
  },
};

export default App;