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
  const min = Math.floor(seconds / 60);
  const sec = Math.floor(seconds % 60);
  return `${min}:${sec.toString().padStart(2, "0")}`;
};

const VideoList: React.FC<VideoListProps> = ({ videos, onSelect, selectedTitle }) => {
	const list = videos ?? [];

	return (
		<div style={{ padding: 16 }}>
			<h2>Available Videos</h2>
			{list.length === 0 ? (
				<div>No videos uploaded yet.</div>
			) : (
				<ul style={{ listStyle: "none", padding: 0 }}>
					{list.map((video) => (
          <li
            key={video.title}
            onClick={() => onSelect(video)}
            style={{
              padding: "12px 16px",
              marginBottom: 8,
              background: video.title === selectedTitle ? "#e0e7ff" : "#f3f4f6",
              borderRadius: 6,
              cursor: "pointer",
              display: "flex",
              justifyContent: "space-between",
              alignItems: "center",
            }}
          >
            <span>{video.title}</span>
            <span style={{ color: "#666", fontSize: 14 }}>
              {formatDuration(video.duration)}
            </span>
          </li>
          		))}
				</ul>
			)}
		</div>
	);
};

export default VideoList;
