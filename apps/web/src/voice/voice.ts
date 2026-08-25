import type {
	AnswerPayload,
	CandidatePayload,
	OfferPayload,
	PlayerStatePayload,
	SignalingMessage,
	SignalingSocketClient,
	WorldSocketClient,
} from "../net/websocket";
import { loadIceServers } from "./ice";

type VoiceChatOptions = {
	worldSocket: WorldSocketClient;
	signalingSocket?: SignalingSocketClient;
	onStatus?: (status: VoiceStatus) => void;
	onPeerStateChange?: (state: VoicePeerState) => void;
};

type PeerEntry = {
	// 1 人の相手との WebRTC 接続。
	peer: RTCPeerConnection;
	// 相手の音声を再生するための audio 要素。
	audio: HTMLAudioElement;
	// remoteDescription 設定前に届いた ICE candidate を一時保存する。
	pendingCandidates: RTCIceCandidateInit[];
	// 同じ local track を同じ peer に二重追加しないためのフラグ。
	localTracksAdded: boolean;
	// offer 衝突時に譲る側。ID が大きい client を polite にして、全員で同じ判定に揃える。
	polite: boolean;
	makingOffer: boolean;
	ignoreOffer: boolean;
	negotiationQueued: boolean;
	retryTimer: number | undefined;
};

type PendingSignalingMessage =
	| { type: "offer"; payload: OfferPayload }
	| { type: "answer"; payload: AnswerPayload }
	| { type: "candidate"; payload: CandidatePayload };

// 距離に応じた音量減衰の基準。
// 2m 以内は最大音量、14m 以上はほぼ無音になるようにしている。
const FULL_VOLUME_DISTANCE = 2;
const SILENT_DISTANCE = 14;
const MAX_NEGOTIATION_RETRIES = 3;
const RETRY_BASE_DELAY_MS = 800;
const DISCONNECTED_RETRY_DELAY_MS = 3000;

// 音声 UI へ伝える状態。
// 表示文言への変換は connectionStatus.ts に任せる。
export const VOICE_STATUS = {
	ICE_REQUESTING: "ice requesting",
	MIC_REQUESTING: "mic requesting",
	MIC_ON: "mic on",
	MIC_OFF: "mic off",
	TAP_MIC_TO_START_AUDIO: "tap mic to start audio",
	MIC_ERROR: "mic error",
} as const;

export type VoiceStatus = (typeof VOICE_STATUS)[keyof typeof VOICE_STATUS];
export type VoicePeerState = "idle" | "connecting" | "connected" | "failed";

// WebRTC 音声通話を扱う部品。
// 音声データ自体は RTCPeerConnection に流し、WebSocket は接続準備の signaling にだけ使う。
export function createVoiceChat({
	worldSocket,
	signalingSocket: initialSignalingSocket,
	onStatus,
	onPeerStateChange,
}: VoiceChatOptions) {
	// user ID -> その相手との WebRTC 接続。
	const peers = new Map<string, PeerEntry>();
	// 同じ相手に対して同時に PeerConnection を作らないための作成中 Promise。
	const peerCreationPromises = new Map<string, Promise<PeerEntry>>();
	// 相手の位置情報。距離に応じた音量調整に使う。
	const remotePositions = new Map<string, PlayerStatePayload>();
	const negotiationRetries = new Map<string, number>();
	let localUserID: string | undefined;
	let localStream: MediaStream | undefined;
	let localX = 0;
	let localY = 0;
	let enabled = false;
	let signalingSocket = initialSignalingSocket;
	const pendingSignalingMessages: PendingSignalingMessage[] = [];
	let iceServersPromise: Promise<RTCIceServer[]> | undefined;
	let currentPeerState: VoicePeerState = "idle";

	function ensureIceServers() {
		// ICE server 設定は一度取れたら使い回す。
		// 失敗した時は undefined に戻して、次回 enable 時に再試行できるようにする。
		if (!iceServersPromise) {
			iceServersPromise = loadIceServers().catch((error) => {
				iceServersPromise = undefined;
				throw error;
			});
		}

		return iceServersPromise;
	}

	function setStatus(status: VoiceStatus) {
		onStatus?.(status);
	}

	function updatePeerState() {
		// 複数 peer がある場合、1 人でも connected なら通話可能として扱う。
		// 全員失敗している時だけ failed を出す。
		let newState: VoicePeerState = "idle";

		if (peers.size === 0) {
			newState = "idle";
		} else {
			let hasConnected = false;
			let hasConnecting = false;
			let hasFailed = false;

			for (const { peer } of peers.values()) {
				const state = peer.connectionState;
				// RTCPeerConnectionState は "new" や "disconnected" も来る。
				// UI では細かすぎるので、このアプリ用の 4 状態に丸める。
				if (state === "connected") {
					hasConnected = true;
				} else if (state === "new" || state === "connecting") {
					hasConnecting = true;
				} else if (
					state === "failed" ||
					state === "closed" ||
					state === "disconnected"
				) {
					hasFailed = true;
				}
			}

			if (hasConnected) {
				newState = "connected";
			} else if (hasConnecting) {
				newState = "connecting";
			} else if (hasFailed) {
				newState = "failed";
			}
		}

		// 状態が変わった時だけ UI に通知する。
		if (currentPeerState !== newState) {
			currentPeerState = newState;
			onPeerStateChange?.(newState);
		}
	}

	async function enable() {
		if (enabled) {
			return;
		}

		// まず ICE server 設定を取得し、その後にマイク許可を求める。
		// マイク取得に成功したら speak_start を送って、他プレイヤーにも状態を伝える。
		setStatus(VOICE_STATUS.ICE_REQUESTING);
		await ensureIceServers();
		setStatus(VOICE_STATUS.MIC_REQUESTING);
		localStream = await navigator.mediaDevices.getUserMedia({
			audio: true,
		});
		enabled = true;
		setStatus(VOICE_STATUS.MIC_ON);
		worldSocket.send("speak_start", {});

		// 既に peer がある場合は、取得したマイク track を追加して offer を作り直す。
		for (const [id, entry] of peers) {
			addLocalTracks(entry);
			void requestNegotiation(id, true);
		}

		// 位置だけ先に知っている相手にも、必要ならこちらから offer を始める。
		for (const id of remotePositions.keys()) {
			maybeStartOffer(id);
		}
	}

	function disable() {
		// 既に無効なら何もしない。
		if (!enabled && !localStream) {
			return;
		}

		// マイク track を止め、全 peer を閉じる。
		// speak_stop を送ることで、他プレイヤーの表示も「話していない」状態になる。
		enabled = false;
		worldSocket.send("speak_stop", {});
		localStream?.getTracks().forEach((track) => {
			track.stop();
		});
		localStream = undefined;

		for (const id of peers.keys()) {
			closePeer(id);
		}

		setStatus(VOICE_STATUS.MIC_OFF);
	}

	function setLocalUserID(id: string) {
		// signaling の衝突回避や、自分自身を相手として扱わない判定に使う。
		localUserID = id;
	}

	function setSignalingSocket(socket: SignalingSocketClient) {
		signalingSocket = socket;
		flushSignalingMessages();
	}

	function flushSignalingMessages() {
		if (!signalingSocket?.isOpen()) {
			return;
		}

		const messages = pendingSignalingMessages.splice(0);
		for (const message of messages) {
			signalingSocket.send(message.type, message.payload);
		}
	}

	function sendSignaling(message: PendingSignalingMessage) {
		if (!signalingSocket?.isOpen()) {
			pendingSignalingMessages.push(message);
			return;
		}

		signalingSocket.send(message.type, message.payload);
	}

	function upsertRemotePlayer(player: PlayerStatePayload) {
		// 相手の位置を保存し、距離に応じた音量を更新する。
		// マイクが ON なら、必要に応じて WebRTC 接続も開始する。
		remotePositions.set(player.u, player);
		updateVolume(player.u);
		maybeStartOffer(player.u);
	}

	function syncRemotePlayers(players: PlayerStatePayload[]) {
		// サーバーの参加者一覧と voice 側の相手一覧を揃える。
		// 一覧にいない相手は退出済みなので peer も閉じる。
		const seen = new Set<string>();

		for (const player of players) {
			if (player.u === localUserID) {
				continue;
			}

			seen.add(player.u);
			upsertRemotePlayer(player);
		}

		for (const id of remotePositions.keys()) {
			if (!seen.has(id)) {
				removeRemotePlayer(id);
			}
		}
	}

	function removeRemotePlayer(id: string) {
		// 相手が退出したら位置情報と WebRTC 接続を両方消す。
		remotePositions.delete(id);
		negotiationRetries.delete(id);
		closePeer(id);
	}

	function updateLocalPosition(x: number, y: number) {
		// 自分の位置が変わるたびに全相手の音量を再計算する。
		localX = x;
		localY = y;

		for (const id of peers.keys()) {
			updateVolume(id);
		}
	}

	async function handleMessage(message: SignalingMessage) {
		// offer/answer/candidate は通話用なのでここで処理し、処理できたら true を返す。
		// room.ts は true の時、ゲーム用メッセージとしては扱わない。
		if (message.t === "offer") {
			await handleOffer(message);
			return true;
		}

		if (message.t === "answer") {
			await handleAnswer(message);
			return true;
		}

		if (message.t === "candidate") {
			await handleCandidate(message);
			return true;
		}

		return false;
	}

	function maybeStartOffer(remoteID: string) {
		// マイク OFF、自分の ID 未確定、自分自身なら offer は不要。
		if (
			!enabled ||
			!localUserID ||
			remoteID === localUserID
		) {
			return;
		}

		// 通常の offer 開始側は user ID が小さい方。
		// 片側だけ speaking の時は発話側からも開始し、衝突は polite peer 側の rollback で吸収する。
		const remote = remotePositions.get(remoteID);
		if (localUserID < remoteID || remote?.speaking === false) {
			void requestNegotiation(remoteID);
		}
	}

	async function requestNegotiation(remoteID: string, force = false) {
		const entry = await ensurePeerEntry(remoteID);
		if (!force && entry.peer.connectionState === "connected") {
			return;
		}

		if (entry.makingOffer || entry.peer.signalingState !== "stable") {
			entry.negotiationQueued = true;
			return;
		}

		await startOffer(remoteID, entry);
	}

	async function startOffer(remoteID: string, entry: PeerEntry) {
		// こちらから相手へ接続提案を送る。
		// SDP は「どんな音声/通信条件でつながりたいか」の説明文。
		entry.makingOffer = true;
		entry.negotiationQueued = false;

		try {
			const offer = await entry.peer.createOffer();
			if (entry.peer.signalingState !== "stable") {
				entry.negotiationQueued = true;
				return;
			}
			await entry.peer.setLocalDescription(offer);

			sendSignaling({
				type: "offer",
				payload: {
					target: remoteID,
					sdp: offer.sdp ?? "",
				},
			});
		} catch (error) {
			console.warn("offer negotiation failed", remoteID, error);
			scheduleNegotiationRetry(remoteID, "offer failed");
		} finally {
			entry.makingOffer = false;
		}
	}

	async function handleOffer(message: SignalingMessage) {
		// 相手から offer が来たら remoteDescription に設定し、answer を返す。
		if (!message.u) {
			return;
		}

		const payload = message.d as OfferPayload;
		const entry = await ensurePeerEntry(message.u);
		const peer = entry.peer;
		const offerCollision =
			entry.makingOffer || peer.signalingState !== "stable";
		entry.ignoreOffer = !entry.polite && offerCollision;

		if (entry.ignoreOffer) {
			return;
		}

		try {
			if (offerCollision) {
				await peer.setLocalDescription({ type: "rollback" });
			}

			await peer.setRemoteDescription({
				type: "offer",
				sdp: payload.sdp,
			});

			addLocalTracks(entry);
			const answer = await peer.createAnswer();
			await peer.setLocalDescription(answer);

			sendSignaling({
				type: "answer",
				payload: {
					target: message.u,
					sdp: answer.sdp ?? "",
				},
			});

			resetNegotiationRetry(entry);
			entry.ignoreOffer = false;
			await flushPendingCandidates(message.u);
		} catch (error) {
			console.warn("offer handling failed", message.u, error);
			scheduleNegotiationRetry(message.u, "offer handling failed");
		}
	}

	async function handleAnswer(message: SignalingMessage) {
		// こちらが送った offer への返事。
		// remoteDescription が入ると、保留していた ICE candidate も追加できる。
		if (!message.u) {
			return;
		}

		const entry = peers.get(message.u);
		if (!entry) {
			return;
		}
		if (entry.peer.signalingState !== "have-local-offer") {
			return;
		}

		const payload = message.d as AnswerPayload;
		try {
			await entry.peer.setRemoteDescription({
				type: "answer",
				sdp: payload.sdp,
			});

			resetNegotiationRetry(entry);
			entry.ignoreOffer = false;
			await flushPendingCandidates(message.u);
		} catch (error) {
			console.warn("answer handling failed", message.u, error);
			scheduleNegotiationRetry(message.u, "answer handling failed");
		}
	}

	async function handleCandidate(message: SignalingMessage) {
		// ICE candidate は、どの経路で相手に到達できそうかという候補。
		// remoteDescription より先に届くことがあるので、その場合は一旦保留する。
		if (!message.u) {
			return;
		}

		const payload = message.d as CandidatePayload;
		const candidate = {
			candidate: payload.candidate,
			sdpMid: payload.sdpMid,
			sdpMLineIndex: payload.sdpMLineIndex,
		};

		const entry = await ensurePeerEntry(message.u);

		if (!entry.peer.remoteDescription) {
			if (entry.ignoreOffer) {
				return;
			}
			entry.pendingCandidates.push(candidate);
			return;
		}

		await addIceCandidate(entry, message.u, candidate);
	}

	async function ensurePeerEntry(remoteID: string): Promise<PeerEntry> {
		// 既にある接続は再利用する。
		const current = peers.get(remoteID);
		if (current) {
			return current;
		}

		const pending = peerCreationPromises.get(remoteID);
		if (pending) {
			// 作成中の promise を返すことで、同時呼び出しでも peer が 1 つだけになる。
			return pending;
		}

		const creation = createPeerEntry(remoteID).finally(() => {
			peerCreationPromises.delete(remoteID);
		});
		peerCreationPromises.set(remoteID, creation);

		return creation;
	}

	async function createPeerEntry(remoteID: string): Promise<PeerEntry> {
		// TURN/STUN 設定を使って WebRTC 接続を作る。
		const peer = new RTCPeerConnection({
			iceServers: await ensureIceServers(),
		});

		// 相手の音声を再生する audio 要素を動的に作る。
		// 画面に見える UI ではないが、ブラウザで音を鳴らすために DOM へ追加する。
		const audio = document.createElement("audio");
		audio.autoplay = true;
		audio.setAttribute("playsinline", "true");
		audio.dataset.remoteUserId = remoteID;
		document.body.append(audio);
		let entry: PeerEntry;

		peer.addEventListener("icecandidate", (event) => {
			// ブラウザが候補経路を見つけるたびに相手へ送る。
			if (!event.candidate) {
				return;
			}

			sendSignaling({
				type: "candidate",
				payload: {
					target: remoteID,
					candidate: event.candidate.candidate,
					sdpMid: event.candidate.sdpMid ?? "",
					sdpMLineIndex: event.candidate.sdpMLineIndex ?? 0,
				},
			});
		});

		peer.addEventListener("signalingstatechange", () => {
			if (peer.signalingState === "stable" && entry.negotiationQueued) {
				void requestNegotiation(remoteID);
			}
		});

		peer.addEventListener("track", (event) => {
			// 相手の音声 track が届いたら audio 要素へ流す。
			const [stream] = event.streams;
			if (!stream) {
				return;
			}

			audio.srcObject = stream;
			void audio.play().catch(() => {
				// ブラウザの自動再生制限で再生できない場合は、ユーザー操作を促す表示にする。
				setStatus(VOICE_STATUS.TAP_MIC_TO_START_AUDIO);
			});
		});

		peer.addEventListener("connectionstatechange", () => {
			// peer の状態が変わるたびに、全体の voice 状態を再計算する。
			updatePeerState();
			if (peer.connectionState === "connected") {
				resetNegotiationRetry(entry);
				return;
			}

			if (peer.connectionState === "failed") {
				scheduleNegotiationRetry(remoteID, "connection failed");
				return;
			}

			if (peer.connectionState === "disconnected") {
				scheduleNegotiationRetry(remoteID, "connection disconnected");
				return;
			}

			if (peer.connectionState === "closed") {
				closePeer(remoteID);
			}
		});

		entry = {
			peer,
			audio,
			pendingCandidates: [],
			localTracksAdded: false,
			polite: isPolitePeer(remoteID),
			makingOffer: false,
			ignoreOffer: false,
			negotiationQueued: false,
			retryTimer: undefined,
		};
		addLocalTracks(entry);
		peers.set(remoteID, entry);
		updateVolume(remoteID);
		updatePeerState();
		return entry;
	}

	function addLocalTracks(entry: PeerEntry) {
		// localStream がある時だけ、自分のマイク track を peer に追加する。
		// localTracksAdded で二重追加を防ぐ。
		const stream = localStream;
		if (!stream || entry.localTracksAdded) {
			return;
		}

		stream.getTracks().forEach((track) => {
			entry.peer.addTrack(track, stream);
		});
		entry.localTracksAdded = true;
	}

	async function flushPendingCandidates(remoteID: string) {
		// remoteDescription 設定前に来ていた candidate をまとめて追加する。
		const entry = peers.get(remoteID);
		if (!entry || !entry.peer.remoteDescription) {
			return;
		}

		const candidates = entry.pendingCandidates.splice(0);
		for (const candidate of candidates) {
			await addIceCandidate(entry, remoteID, candidate);
		}
	}

	async function addIceCandidate(
		entry: PeerEntry,
		remoteID: string,
		candidate: RTCIceCandidateInit,
	) {
		try {
			await entry.peer.addIceCandidate(candidate);
		} catch (error) {
			if (!entry.ignoreOffer) {
				console.warn("ice candidate add failed", remoteID, error);
			}
		}
	}

	function closePeer(remoteID: string) {
		// peer を閉じ、追加した audio 要素も DOM から外す。
		const entry = peers.get(remoteID);
		if (!entry) {
			return;
		}

		clearRetryTimer(entry);
		peers.delete(remoteID);
		peerCreationPromises.delete(remoteID);
		entry.peer.close();
		entry.audio.remove();
		updatePeerState();
	}

	function scheduleNegotiationRetry(remoteID: string, reason: string) {
		const entry = peers.get(remoteID);
		if (!entry || !enabled || !remotePositions.has(remoteID)) {
			return;
		}
		if (entry.retryTimer !== undefined) {
			return;
		}
		const retryCount = negotiationRetries.get(remoteID) ?? 0;
		if (retryCount >= MAX_NEGOTIATION_RETRIES) {
			console.warn("voice negotiation retry exhausted", remoteID, reason);
			closePeer(remoteID);
			return;
		}

		const nextRetryCount = retryCount + 1;
		negotiationRetries.set(remoteID, nextRetryCount);
		const baseDelay =
			reason === "connection disconnected"
				? DISCONNECTED_RETRY_DELAY_MS
				: RETRY_BASE_DELAY_MS;
		const delay = baseDelay * nextRetryCount;

		entry.retryTimer = window.setTimeout(() => {
			entry.retryTimer = undefined;
			closePeer(remoteID);
			if (localUserID && localUserID < remoteID) {
				void requestNegotiation(remoteID);
			}
		}, delay);
	}

	function resetNegotiationRetry(entry: PeerEntry) {
		for (const [remoteID, current] of peers) {
			if (current === entry) {
				negotiationRetries.delete(remoteID);
				break;
			}
		}
		clearRetryTimer(entry);
	}

	function clearRetryTimer(entry: PeerEntry) {
		if (entry.retryTimer === undefined) {
			return;
		}
		window.clearTimeout(entry.retryTimer);
		entry.retryTimer = undefined;
	}

	function isPolitePeer(remoteID: string) {
		if (!localUserID) {
			return false;
		}
		return localUserID > remoteID;
	}

	function updateVolume(remoteID: string) {
		// 相手との距離から音量を決める。
		// 近いほど 1 に近く、遠いほど 0 に近づく。
		const entry = peers.get(remoteID);
		const remote = remotePositions.get(remoteID);
		if (!entry || !remote) {
			return;
		}

		const distance = Math.hypot(remote.x - localX, remote.y - localY);
		const t = Math.min(
			Math.max(
				(distance - FULL_VOLUME_DISTANCE) /
					(SILENT_DISTANCE - FULL_VOLUME_DISTANCE),
				0,
			),
			1,
		);
		entry.audio.volume = (1 - t) ** 2;
	}

	return {
		enable,
		disable,
		setLocalUserID,
		setSignalingSocket,
		flushSignalingMessages,
		upsertRemotePlayer,
		syncRemotePlayers,
		removeRemotePlayer,
		updateLocalPosition,
		handleMessage,
	};
}
