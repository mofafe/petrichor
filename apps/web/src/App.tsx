import { useEffect, useRef } from "react";
import { ROOM_PATH } from "./app/constants";
import { startEntryView } from "./app/entry";
import { startRoomView } from "./app/room";

export function App() {
	const canvasRef = useRef<HTMLCanvasElement>(null);
	const startedRef = useRef(false);
	const isRoom = window.location.pathname === ROOM_PATH;

	useEffect(() => {
		if (startedRef.current) {
			return;
		}
		startedRef.current = true;

		document.body.dataset.view = isRoom ? "room" : "entry";
		const canvas = canvasRef.current;
		if (!canvas) {
			throw new Error("Missing scene canvas");
		}

		if (isRoom) {
			startRoomView(canvas);
			return;
		}

		startEntryView();
	}, [isRoom]);

	return (
		<>
			<EntryView />
			<canvas id="scene" ref={canvasRef} />
			<RoomOverlay />
			<div className="hint">Click to look · WASD to move</div>
			<RoomNamePrompt />
			<RoomFullNotice />
		</>
	);
}

function EntryView() {
	return (
		<main className="entry">
			<section className="entry-panel">
				<h1>Iolite UI Lab</h1>
				<label>
					<span>Name</span>
					<input
						id="entry-name"
						type="text"
						maxLength={24}
						defaultValue="guest"
					/>
				</label>
				<label>
					<span>Room</span>
					<input
						id="entry-room"
						type="text"
						maxLength={64}
						defaultValue="debug"
					/>
				</label>
				<button id="entry-join" type="button">
					Join
				</button>
			</section>
		</main>
	);
}

function RoomOverlay() {
	return (
		<div className="overlay" aria-label="Iolite controls">
			<div className="brand">
				<span className="status-dot" />
				<div>
					<h1>Iolite UI Lab</h1>
					<p>pixel-depth voice space</p>
				</div>
			</div>

			<div className="controls">
				<button id="mic-toggle" type="button">
					Mic Off
				</button>
				<button type="button" className="invite">
					Invite
				</button>
				<button type="button" className="leave">
					Leave
				</button>
			</div>
			<p id="voice-status" className="voice-status">
				mic off
			</p>
		</div>
	);
}

function RoomNamePrompt() {
	return (
		<div id="room-name-prompt" className="room-name-prompt" hidden>
			<form id="room-name-form" className="entry-panel">
				<h1>Join Room</h1>
				<label>
					<span>Name</span>
					<input
						id="room-name-input"
						type="text"
						maxLength={24}
						autoComplete="name"
					/>
				</label>
				<button type="submit">Enter Room</button>
			</form>
		</div>
	);
}

function RoomFullNotice() {
	return (
		<div id="room-full" className="room-full-notice" hidden>
			<p>このRoomは満員です（最大6人）。</p>
		</div>
	);
}
