import React from "react";

export interface VideoMeta {
  title: string;
  duration: number;
}

interface VideoListProps {
  videos?: VideoMeta[] | null;
  onSelect: (video: VideoMeta) => void;
  selectedTitle?: string;
}

const formatDuration = (seconds: number) => {
  if (!seconds) return "--:--";
  const min = Math.floor(seconds / 60);
  const sec = Math.floor(seconds % 60);
  return `${min}:${sec.toString().padStart(2, "0")}`;
};

const shortenTitle = (title: string) => {
  const parts = title.split("-");
  // remove trailing timestamp
  if (parts.length > 1 && parts[parts.length - 1].length === 10) {
    return parts.slice(0, -1).join("-");
  }
  return title;
};

const VideoList: React.FC<VideoListProps> = ({ videos, onSelect, selectedTitle }) => {
  const list = videos ?? [];

  if (list.length === 0) {
    return (
      <div style={styles.empty}>
        <div style={styles.emptyIcon}>▭</div>
        <div style={styles.emptyText}>NO CONTENT</div>
        <div style={styles.emptySubtext}>Upload a video to begin streaming</div>
      </div>
    );
  }

  return (
    <div style={styles.list}>
      {list.map((video, idx) => {
        const isSelected = video.title === selectedTitle;
        return (
          <div
            key={video.title}
            onClick={() => onSelect(video)}
            style={{
              ...styles.item,
              ...(isSelected ? styles.itemSelected : {}),
            }}
          >
            <div style={styles.itemIndex}>
              {isSelected ? (
                <span style={styles.playingIcon}>▶</span>
              ) : (
                <span style={styles.indexNum}>{String(idx + 1).padStart(2, "0")}</span>
              )}
            </div>
            <div style={styles.itemInfo}>
              <div style={{
                ...styles.itemTitle,
                ...(isSelected ? styles.itemTitleSelected : {}),
              }}>
                {video.title}
              </div>
              <div style={styles.itemMeta}>
                <span style={styles.metaTag}>HLS</span>
                <span style={styles.metaDot}>·</span>
                <span>{formatDuration(video.duration)}</span>
              </div>
            </div>
            {isSelected && <div style={styles.selectedBar} />}
          </div>
        );
      })}
    </div>
  );
};

const styles: Record<string, React.CSSProperties> = {
  list: {
    overflowY: "auto",
    maxHeight: "400px",
  },
  item: {
    display: "flex",
    alignItems: "center",
    gap: "12px",
    padding: "12px 20px",
    cursor: "pointer",
    borderBottom: "1px solid rgba(0,255,170,0.05)",
    transition: "background 0.15s",
    position: "relative",
    background: "transparent",
  },
  itemSelected: {
    background: "rgba(0,255,170,0.05)",
  },
  itemIndex: {
    width: "20px",
    flexShrink: 0,
    textAlign: "center",
  },
  indexNum: {
    fontSize: "10px",
    color: "rgba(226,232,240,0.2)",
    letterSpacing: "1px",
  },
  playingIcon: {
    fontSize: "10px",
    color: "#00ffaa",
  },
  itemInfo: {
    flex: 1,
    overflow: "hidden",
  },
  itemTitle: {
    fontSize: "12px",
    letterSpacing: "0.5px",
    color: "rgba(226,232,240,0.7)",
    overflow: "hidden",
    textOverflow: "ellipsis",
    whiteSpace: "nowrap",
    marginBottom: "3px",
  },
  itemTitleSelected: {
    color: "#e2e8f0",
  },
  itemMeta: {
    display: "flex",
    alignItems: "center",
    gap: "6px",
    fontSize: "10px",
    color: "rgba(226,232,240,0.3)",
    letterSpacing: "1px",
  },
  metaTag: {
    background: "rgba(0,255,170,0.1)",
    color: "rgba(0,255,170,0.6)",
    padding: "1px 4px",
    borderRadius: "1px",
    fontSize: "9px",
  },
  metaDot: {
    opacity: 0.4,
  },
  selectedBar: {
    position: "absolute",
    left: 0,
    top: 0,
    bottom: 0,
    width: "2px",
    background: "#00ffaa",
    boxShadow: "0 0 8px #00ffaa",
  },
  empty: {
    display: "flex",
    flexDirection: "column",
    alignItems: "center",
    justifyContent: "center",
    padding: "40px 20px",
    gap: "8px",
  },
  emptyIcon: {
    fontSize: "24px",
    color: "rgba(0,255,170,0.2)",
    marginBottom: "4px",
  },
  emptyText: {
    fontSize: "11px",
    letterSpacing: "3px",
    color: "rgba(226,232,240,0.3)",
  },
  emptySubtext: {
    fontSize: "10px",
    color: "rgba(226,232,240,0.2)",
    textAlign: "center",
  },
};

export default VideoList;