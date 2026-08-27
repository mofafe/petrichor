import { useRef, useState } from "react";

// WebRTC の接続確認用ページ。
// 本編の room.ts/voice.ts とは独立して、手動ボタンで WebSocket と通話処理を試すための URL。
const debugUserID = crypto.randomUUID();
const wsProtocol = window.location.protocol === "https:" ? "wss:" : "ws:";
const wsUrl = `${wsProtocol}//${window.location.host}/ws/rooms/signaling/debug?userID=${encodeURIComponent(debugUserID)}`;

export function DebugApp() {
	// サーバーから届く WebSocket message の最小形。
	// 本編と同じく `{ t, u?, d }` という protocol を想定している。
	type ServerMessage = {
		t: string;
		u?: string;
		d: unknown;
	};

	type OfferPayload = {
		sdp: string;
	};

	type AnswerPayload = {
		sdp: string;
	};

	type CandidatePayload = {
		candidate: string;
		sdpMid: string;
		sdpMLineIndex: number;
	};

	// React の state は、画面に表示したい値を持つ。
	// setStatus() などで更新すると React が再描画して画面に反映する。
	const [status, setStatus] = useState("idle");
	const [logs, setLogs] = useState<string[]>([]);

	// useRef は、再描画しても同じ値を持ち続けたい時に使う。
	// WebSocket や RTCPeerConnection は React の表示値ではなく、ブラウザ API の実体なので ref に入れる。
	const wsRef = useRef<WebSocket | null>(null);
	const [micStatus, setMicStatus] = useState("mic idle");
	const localStreamRef = useRef<MediaStream | null>(null);
	const [peerStatus, setPeerStatus] = useState("peer idle");
	const peerRef = useRef<RTCPeerConnection | null>(null);
	const remoteUserIdRef = useRef<string | null>(null);
	const remoteAudioRef = useRef<HTMLAudioElement | null>(null);
	const pendingCandidatesRef = useRef<RTCIceCandidate[]>([]);

	function log(message: string) {
		// ログは末尾へ追加していく。
		// currentLogs を使う形にすると、連続更新でも古い logs を取りこぼしにくい。
		setLogs((currentLogs) => [...currentLogs, message]);
	}

	function handleConnect() {
		// 既に WebSocket が開いているなら作り直さない。
		if (wsRef.current?.readyState === WebSocket.OPEN) {
			log("WebSocket is already open");
			return;
		}

		setStatus("connecting");
		log(`connecting to ${wsUrl}`);

		const ws = new WebSocket(wsUrl);
		wsRef.current = ws;

		// WebSocket の状態変化を画面表示とログへ反映する。
		ws.addEventListener("open", () => {
			setStatus("connected");
			log("WebSocket connected");
		});

		ws.addEventListener("message", (event) => {
			void handleMessage(event);
		});

		ws.addEventListener("error", () => {
			setStatus("error");
			log("WebSocket error");
		});

		ws.addEventListener("close", () => {
			setStatus("closed");
			log("WebSocket closed");
			wsRef.current = null;
		});
	}

	function handleLeave() {
		// デバッグ中に作った WebSocket / peer / mic をまとめて片付ける。
		// ブラウザ API は明示的に close/stop しないと接続やマイクが残ることがある。
		wsRef.current?.close();
		wsRef.current = null;

		peerRef.current?.close();
		peerRef.current = null;

		localStreamRef.current?.getTracks().forEach((track) => {
			track.stop();
		});
		localStreamRef.current = null;

		remoteUserIdRef.current = null;

		if (remoteAudioRef.current) {
			remoteAudioRef.current.srcObject = null;
		}

		pendingCandidatesRef.current = [];

		setStatus("closed");
		setMicStatus("mic idle");
		setPeerStatus("peer closed");

		log("left debug session");
	}

	async function handleStartMic() {
		try {
			// ブラウザにマイク許可を求める。
			// 成功すると MediaStream が返り、後で peer.addTrack() で相手へ送れる。
			setMicStatus("requesting mic");
			log("Requesting microphone");

			const stream = await navigator.mediaDevices.getUserMedia({
				audio: true,
			});

			localStreamRef.current = stream;
			setMicStatus("mic ready");
			log(
				`Microphone ready: ${stream.getAudioTracks().length} audio track`,
			);
		} catch (error) {
			setMicStatus("mic error");
			log(`Microphone error: ${String(error)}`);
		}
	}

	function createPeer() {
		// 既に peer がある場合は再利用する。
		// 1 回のデバッグ通話では 1 つの RTCPeerConnection を使う想定。
		if (peerRef.current) {
			return peerRef.current;
		}

		const peer = new RTCPeerConnection();

		// 接続状態を細かくログへ出す。
		// WebRTC は失敗箇所が分かりにくいので、state の変化を見ることが大事。
		peer.addEventListener("connectionstatechange", () => {
			setPeerStatus(peer.connectionState);
			log(`peer connection state: ${peer.connectionState}`);
		});

		peer.addEventListener("iceconnectionstatechange", () => {
			log(`ice connection state: ${peer.iceConnectionState}`);
		});

		peer.addEventListener("signalingstatechange", () => {
			log(`signaling state: ${peer.signalingState}`);
		});

		peer.addEventListener("icecandidate", (event) => {
			// ICE candidate は相手へ到達できる可能性がある経路情報。
			// 相手の user ID がまだ分からない時は、後で送るため pending に積む。
			if (!event.candidate) {
				log("ice candidate gathering complete");
				return;
			}

			const target = remoteUserIdRef.current;

			if (!target) {
				pendingCandidatesRef.current.push(event.candidate);
				log("ice candidate queued");
				return;
			}

			send({
				t: "candidate",
				d: {
					target,
					candidate: event.candidate.candidate,
					sdpMid: event.candidate.sdpMid ?? "",
					sdpMLineIndex: event.candidate.sdpMLineIndex ?? 0,
				},
			});

			log("ice candidate sent");
		});

		peer.addEventListener("track", (event) => {
			// 相手の音声 track が届いたら audio 要素へ流す。
			const [remoteStream] = event.streams;

			if (!remoteStream) {
				log("remote track received without stream");
				return;
			}

			if (remoteAudioRef.current) {
				remoteAudioRef.current.srcObject = remoteStream;
			}

			log(`remote track received: ${event.track.kind}`);
		});

		const localStream = localStreamRef.current;

		// マイクが準備済みなら、自分の音声 track を peer に追加する。
		// まだ Start Mic していない場合はログだけ出して、後で原因を追いやすくする。
		if (localStream) {
			localStream.getTracks().forEach((track) => {
				peer.addTrack(track, localStream);
			});

			log(`added local tracks: ${localStream.getTracks().length}`);
		} else {
			log("local stream is not ready");
		}

		peerRef.current = peer;
		setPeerStatus(peer.connectionState);

		return peer;
	}

	function send(message: unknown) {
		// WebSocket が開いている時だけ JSON として送る。
		// 開いていない時は送れない理由をログに残す。
		const ws = wsRef.current;

		if (!ws || ws.readyState !== WebSocket.OPEN) {
			log("WebSocket is not open");
			return;
		}

		const text = JSON.stringify(message);
		ws.send(text);
		log(`sent: ${text}`);
	}

	async function handleCall() {
		// 自分から通話を始める時は offer を作って WebSocket で送る。
		// 相手はこの offer を受けて answer を返す。
		const peer = createPeer();

		const offer = await peer.createOffer();
		await peer.setLocalDescription(offer);

		send({
			t: "offer",
			d: {
				sdp: offer.sdp,
			},
		});
	}

	async function handleMessage(event: MessageEvent<string>) {
		// サーバーから来た JSON を見て、WebRTC signaling の種類ごとに分岐する。
		log(`received: ${event.data}`);

		const message = JSON.parse(event.data) as ServerMessage;

		if (message.t === "offer") {
			await handleOffer(message);
		}

		if (message.t === "answer") {
			await handleAnswer(message);
		}

		if (message.t === "candidate") {
			await handleCandidate(message);
			return;
		}
	}

	async function handleOffer(message: ServerMessage) {
		// 相手から offer が来た時は remoteDescription に設定し、answer を作って返す。
		if (!message.u) {
			log("offer sender is missing");
			return;
		}

		const payload = message.d as OfferPayload;
		const peer = createPeer();

		await peer.setRemoteDescription({
			type: "offer",
			sdp: payload.sdp,
		});

		const answer = await peer.createAnswer();
		await peer.setLocalDescription(answer);

		send({
			t: "answer",
			d: {
				target: message.u,
				sdp: answer.sdp,
			},
		});

		log(`answered offer from ${message.u}`);
	}

	async function handleAnswer(message: ServerMessage) {
		// 自分が送った offer に対する相手の返事。
		// ここで remoteUserId が分かるので、保留中の candidate も送れる。
		if (!message.u) {
			log("answer sender is missing");
			return;
		}

		remoteUserIdRef.current = message.u;

		const payload = message.d as AnswerPayload;
		const peer = createPeer();

		await peer.setRemoteDescription({
			type: "answer",
			sdp: payload.sdp,
		});

		flushPendingCandidates();
	}

	async function handleCandidate(message: ServerMessage) {
		// 相手から届いた ICE candidate を peer に追加する。
		// これによりブラウザ同士が実際につながる経路を試せる。
		const payload = message.d as CandidatePayload;
		const peer = createPeer();

		await peer.addIceCandidate({
			candidate: payload.candidate,
			sdpMid: payload.sdpMid,
			sdpMLineIndex: payload.sdpMLineIndex,
		});

		log("ice candidate added");
	}

	function sendCandidate(target: string, candidate: RTCIceCandidate) {
		// ICE candidate をサーバー経由で相手へ届ける。
		send({
			t: "candidate",
			d: {
				target,
				candidate: candidate.candidate,
				sdpMid: candidate.sdpMid ?? "",
				sdpMLineIndex: candidate.sdpMLineIndex ?? 0,
			},
		});

		log("ice candidate sent");
	}

	function flushPendingCandidates() {
		// target が分かった後、貯めていた candidate をまとめて送る。
		const target = remoteUserIdRef.current;

		if (!target) {
			return;
		}

		pendingCandidatesRef.current.forEach((candidate) => {
			sendCandidate(target, candidate);
		});

		pendingCandidatesRef.current = [];
	}

	// この JSX はデバッグ操作用の簡単な UI。
	// 各ボタンを順に押して、WebSocket と WebRTC のどこまで進んだかログで確認する。
	return (
		<main>
			<h1>WebRTC Debug</h1>
			<p>room: debug</p>

			<div>
				<button type="button" onClick={handleConnect}>
					Connect WS
				</button>
				<button type="button">Join</button>
				<button type="button" onClick={handleStartMic}>
					Start Mic
				</button>
				<button type="button" onClick={handleCall}>
					Call
				</button>
				<button type="button" onClick={handleLeave}>
					Leave
				</button>
			</div>

			<p>{status}</p>
			<p>{micStatus}</p>
			<p>{peerStatus}</p>

			<audio ref={remoteAudioRef} autoPlay controls />
			<pre>{logs.join("\n")}</pre>
		</main>
	);
}
