import React, { useEffect, useRef, useState } from "react";
import Hls from "hls.js";

interface VideoPlayerProps {
  title?: string;
}

const VideoPlayer: React.FC<VideoPlayerProps> = ({ title }) => {
  const videoRef = useRef<HTMLVideoElement | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const [stats, setStats] = useState({ buffered: 0, currentTime: 0, duration: 0 });

  useEffect(() => {
    if (!title || !videoRef.current) return;

    const video = videoRef.current;
    setLoading(true);
    setError("");

    const masterUrl = `/hls/${title}/master.m3u8`;
    let hls: Hls | undefined;
    let cancelled = false;

    const load = async () => {
      const exists = async (url: string) => {
        try {
          const res = await fetch(url, { method: "HEAD" });
          return res.ok;
        } catch { return false; }
      };

      const hasMaster = await exists(masterUrl);
      if (cancelled) return;

      if (!hasMaster) {
        setError("Stream not found");
        setLoading(false);
        return;
      }

      if (Hls.isSupported()) {
        hls = new Hls({ lowLatencyMode: true });
        hls.loadSource(masterUrl);
        hls.attachMedia(video);
        hls.on(Hls.Events.MANIFEST_PARSED, () => {
          setLoading(false);
          video.play().catch(() => {});
        });
        hls.on(Hls.Events.ERROR, (_, data) => {
          if (data.fatal) {
            setError("Stream error");
            setLoading(false);
          }
        });
      } else if (video.canPlayType("application/vnd.apple.mpegurl")) {
        video.src = masterUrl;
        setLoading(false);
      }
    };

    load();

    const statsInterval = setInterval(() => {
      if (!video) return;
      const buffered = video.buffered.length > 0
        ? video.buffered.end(video.buffered.length - 1) - video.currentTime
        : 0;
      setStats({
        buffered: Math.max(0, buffered),
        currentTime: video.currentTime,
        duration: video.duration || 0,
      });
    }, 500);

    return () => {
      cancelled = true;
      clearInterval(statsInterval);
      if (hls) hls.destroy();
    };
  }, [title]);

  const formatTime = (s: number) => {
    if (!s || isNaN(s)) return "0:00";
    const m = Math.floor(s / 60);
    const sec = Math.floor(s % 60);
    return `${m}:${sec.toString().padStart(2, "0")}`;
  };

  if (!title) {
    return (
      <div style={styles.empty}>
        <div style={styles.emptyContent}>
          <div style={styles.emptyIcon}>▷</div>
          <div style={styles.emptyTitle}>SELECT A STREAM</div>
          <div style={styles.emptySubtitle}>Choose content from the library to begin playback</div>
          <div style={styles.emptyTags}>
            <span style={styles.tag}>HLS</span>
            <span style={styles.tag}>EDGE CACHED</span>
            <span style={styles.tag}>LOW LATENCY</span>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div style={styles.playerContainer}>
      {/* Title bar */}
      <div style={styles.titleBar}>
        <div style={styles.titleLeft}>
          <span style={styles.titleIcon}>▶</span>
          <span style={styles.titleText}>{title}</span>
        </div>
        <div style={styles.titleRight}>
          <span style={styles.tag}>LIVE EDGE</span>
        </div>
      </div>

      {/* Video */}
      <div style={styles.videoWrapper}>
        {loading && (
          <div style={styles.overlay}>
            <div style={styles.spinner}>◌</div>
            <div style={styles.overlayText}>BUFFERING STREAM</div>
          </div>
        )}
        {error && (
          <div style={styles.overlay}>
            <div style={styles.errorIcon}>✕</div>
            <div style={styles.overlayText}>{error.toUpperCase()}</div>
          </div>
        )}
        <video
          ref={videoRef}
          controls
          style={styles.video}
        />
      </div>

      {/* Stats bar */}
      <div style={styles.statsBar}>
        <div style={styles.stat}>
          <span style={styles.statLabel}>TIME</span>
          <span style={styles.statValue}>{formatTime(stats.currentTime)} / {formatTime(stats.duration)}</span>
        </div>
        <div style={styles.stat}>
          <span style={styles.statLabel}>BUFFER</span>
          <span style={styles.statValue}>{stats.buffered.toFixed(1)}s</span>
        </div>
        <div style={styles.stat}>
          <span style={styles.statLabel}>PROTOCOL</span>
          <span style={styles.statValue}>HLS/fMP4</span>
        </div>
        <div style={styles.stat}>
          <span style={styles.statLabel}>NODE</span>
          <span style={{ ...styles.statValue, color: "#00ffaa" }}>MEC-OULU</span>
        </div>
      </div>
    </div>
  );
};

const styles: Record<string, React.CSSProperties> = {
  empty: {
    flex: 1,
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    height: "100%",
    background: "rgba(0,255,170,0.01)",
  },
  emptyContent: {
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    gap: "12px",
  },
  emptyIcon: {
    fontSize: "48px",
    color: "rgba(0,255,170,0.15)",
    marginBottom: "8px",
  },
  emptyTitle: {
    fontSize: "14px",
    letterSpacing: "4px",
    color: "rgba(226,232,240,0.4)",
  },
  emptySubtitle: {
    fontSize: "11px",
    color: "rgba(226,232,240,0.2)",
    letterSpacing: "1px",
  },
  emptyTags: {
    display: "flex",
    gap: "8px",
    marginTop: "8px",
  },
  tag: {
    padding: "3px 8px",
    border: "1px solid rgba(0,255,170,0.2)",
    borderRadius: "1px",
    fontSize: "9px",
    letterSpacing: "2px",
    color: "rgba(0,255,170,0.4)",
  },
  playerContainer: {
    display: "flex",
    flexDirection: "column",
    height: "100%",
    background: "#0d1117",
  },
  titleBar: {
    display: "flex",
    justifyContent: "space-between",
    alignItems: "center",
    padding: "12px 24px",
    borderBottom: "1px solid rgba(0,255,170,0.1)",
    background: "rgba(0,0,0,0.3)",
  },
  titleLeft: {
    display: "flex",
    alignItems: "center",
    gap: "10px",
  },
  titleIcon: {
    fontSize: "10px",
    color: "#00ffaa",
  },
  titleText: {
    fontSize: "12px",
    letterSpacing: "1px",
    color: "rgba(226,232,240,0.8)",
    maxWidth: "400px",
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
  },
  titleRight: {
    display: "flex",
    gap: "8px",
  },
  videoWrapper: {
    flex: 1,
    position: "relative",
    background: "#000",
    display: "flex",
    alignItems: "center",
    justifyContent: "center",
    overflow: "hidden",
  },
  video: {
    width: "100%",
    height: "100%",
    maxHeight: "calc(100vh - 200px)",
    objectFit: "contain",
  },
  overlay: {
    position: "absolute",
    inset: 0,
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    background: "rgba(8,12,16,0.8)",
    gap: "12px",
    zIndex: 10,
  },
  spinner: {
    fontSize: "32px",
    color: "#00ffaa",
    animation: "spin 1s linear infinite",
  },
  errorIcon: {
    fontSize: "32px",
    color: "rgba(255,60,60,0.8)",
  },
  overlayText: {
    fontSize: "11px",
    letterSpacing: "3px",
    color: "rgba(226,232,240,0.5)",
  },
  statsBar: {
    display: "flex",
    gap: "32px",
    padding: "10px 24px",
    borderTop: "1px solid rgba(0,255,170,0.08)",
    background: "rgba(0,0,0,0.3)",
  },
  stat: {
    display: "flex",
    flexDirection: "column",
    gap: "2px",
  },
  statLabel: {
    fontSize: "9px",
    letterSpacing: "2px",
    color: "rgba(226,232,240,0.3)",
  },
  statValue: {
    fontSize: "11px",
    letterSpacing: "1px",
    color: "rgba(226,232,240,0.7)",
    fontVariantNumeric: "tabular-nums",
  },
};

export default VideoPlayer;