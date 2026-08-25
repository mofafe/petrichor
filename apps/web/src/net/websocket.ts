export type RotationPayload = {
	// Three.js の Euler 回転を JSON で送れる形にしたもの。
	// x は上下、y は左右、z は傾きとして使われる。
	x: number;
	y: number;
	z: number;
};

export type PlayerStatePayload = {
	// サーバーが割り当てるユーザー ID。
	u: string;
	name: string;
	// 床の上の 2D 座標。クライアント内部では x/z だが、通信では x/y として扱う。
	x: number;
	y: number;
	rotation: RotationPayload;
	// 音声発話中かどうか。scene.ts では見た目の色や印に使う。
	speaking: boolean;
};

// 入室時に送る payload。
// サーバーはこれを使って初期位置と名前を保存する。
export type JoinPayload = {
	name: string;
	x: number;
	y: number;
	rotation: RotationPayload;
};

// 移動時に送る payload。
// 名前や speaking は毎回送らず、位置と向きだけにして通信量を小さくしている。
export type MovePayload = {
	x: number;
	y: number;
	rotation: RotationPayload;
};

// サーバーが現在の参加者一覧をまとめて送る時の payload。
export type StateSyncPayload = {
	players: PlayerStatePayload[];
};

// 以下 3 つは WebRTC の通話接続を作るための signaling payload。
// 音声データ本体ではなく、相手と直接つながるための情報だけを WebSocket で交換する。
export type OfferPayload = {
	target?: string;
	sdp: string;
};

export type AnswerPayload = {
	target: string;
	sdp: string;
};

export type CandidatePayload = {
	target: string;
	candidate: string;
	sdpMid: string;
	sdpMLineIndex: number;
};

// サーバーとクライアントで使う world event 名。
export type WorldEventType =
	| "join"
	| "leave"
	| "move"
	| "state_sync"
	| "speak_start"
	| "speak_stop"
	| "ping"
	| "pong"
	| "room_full";

export type SignalingEventType = "offer" | "answer" | "candidate";

export type SocketMessage<
	TEventType extends string,
	TPayload = unknown,
> = {
	// t = type。どのイベントかを表す短いキー。
	t: TEventType;
	// u = user。サーバーから届く時に送信者の user ID が入る。
	u?: string;
	// d = data。イベントごとの payload が入る。
	d: TPayload;
};

export type WorldMessage<TPayload = unknown> = SocketMessage<
	WorldEventType,
	TPayload
>;

export type SignalingMessage<TPayload = unknown> = SocketMessage<
	SignalingEventType,
	TPayload
>;

// createWorldSocket() / createSignalingSocket() の呼び出し側が必要なイベントだけ渡せるよう optional にしている。
type SocketHandlers<TMessage> = {
	open?: () => void;
	message?: (message: TMessage) => void;
	close?: (event: CloseEvent) => void;
	error?: (event: Event) => void;
	onStateChange?: (state: WorldSocketState) => void;
};

// WebSocket の接続状態。
// UI 側はこの値を見て「接続中」「エラー」などを表示する。
export type WorldSocketState =
	| "connecting"
	| "connected"
	| "disconnected"
	| "error";

export type WorldSocketClient = {
	// 型名と payload を渡すと `{ t, d }` の JSON にして送る。
	send: <TPayload>(type: WorldEventType, payload: TPayload) => void;
	close: () => void;
	isOpen: () => boolean;
	getState: () => WorldSocketState;
};

export type SignalingSocketClient = {
	send: <TPayload>(type: SignalingEventType, payload: TPayload) => void;
	close: () => void;
	isOpen: () => boolean;
	getState: () => WorldSocketState;
};

const DEFAULT_ROOM_ID = "debug";
const ROOM_ID_PATTERN = /^[a-zA-Z0-9_-]{1,64}$/;
const LOCAL_WS_HOSTNAME = "localhost";
const LOCAL_WS_PORT = "8080";
const VITE_DEV_PORT_PREFIX = "517";
const LOCAL_HOSTNAMES = new Set(["localhost", "127.0.0.1"]);
const WORLD_WS_PATH = "/ws/rooms/world";
const SIGNALING_WS_PATH = "/ws/rooms/signaling";
const RUNTIME_CONFIG =
	window.__PETRICHOR_CONFIG__ ?? window.__FLATTALKING_CONFIG__;
const WORLD_WS_ORIGIN = RUNTIME_CONFIG?.worldWsOrigin?.trim();

// ページが http なら ws、https なら wss を使う。
// ブラウザは https ページから安全でない ws へ接続するのを嫌うため、ページの protocol に合わせる。
const WS_PROTOCOL_BY_PAGE_PROTOCOL = {
	"http:": "ws:",
	"https:": "wss:",
} as const;

export function roomIDFromLocation(
	location: Location = window.location,
): string {
	// URL の `?room=` を読む。なければ normalizeRoomID() が既定値へ丸める。
	return normalizeRoomID(new URLSearchParams(location.search).get("room"));
}

export function normalizeRoomID(roomID: string | null): string {
	// 空文字や使えない文字を含む room ID は debug に戻す。
	// URL や WebSocket path に入る値なので、使える文字を制限している。
	const normalized = roomID?.trim();
	return normalized && ROOM_ID_PATTERN.test(normalized)
		? normalized
		: DEFAULT_ROOM_ID;
}

export function worldWebSocketURL(
	roomID: string,
	location: Location = window.location,
): string {
	// 本番では runtime config の origin を優先し、ローカルでは現在の location から推測する。
	const origin = WORLD_WS_ORIGIN || defaultWorldWebSocketOrigin(location);
	return `${origin}${WORLD_WS_PATH}/${encodeURIComponent(roomID)}`;
}

export function signalingWebSocketURL(
	roomID: string,
	userID: string,
	location: Location = window.location,
): string {
	const origin = WORLD_WS_ORIGIN || defaultWorldWebSocketOrigin(location);
	const url = new URL(
		`${origin}${SIGNALING_WS_PATH}/${encodeURIComponent(roomID)}`,
	);
	url.searchParams.set("userID", userID);
	return url.toString();
}

function defaultWorldWebSocketOrigin(location: Location): string {
	// Vite dev server から見ると frontend は 517x、backend は 8080 なので後で port を補正する。
	const protocol =
		WS_PROTOCOL_BY_PAGE_PROTOCOL[
			location.protocol as keyof typeof WS_PROTOCOL_BY_PAGE_PROTOCOL
		] ?? "ws:";
	const hostname = location.hostname || LOCAL_WS_HOSTNAME;
	const port = worldWebSocketPort(location, hostname);

	return `${protocol}//${hostname}${port}`;
}

function worldWebSocketPort(location: Location, hostname: string): string {
	const isViteDevServer = location.port.startsWith(VITE_DEV_PORT_PREFIX);
	const isLocalWithoutPort =
		location.port === "" && LOCAL_HOSTNAMES.has(hostname);

	if (isViteDevServer || isLocalWithoutPort) {
		// ローカル開発では Go server 側の 8080 に WebSocket をつなぐ。
		return `:${LOCAL_WS_PORT}`;
	}

	return location.port === "" ? "" : `:${location.port}`;
}

export function createWorldSocket(
	url: string,
	handlers: SocketHandlers<WorldMessage> = {},
): WorldSocketClient {
	return createSocketClient<WorldEventType, WorldMessage>(url, handlers);
}

export function createSignalingSocket(
	url: string,
	handlers: SocketHandlers<SignalingMessage> = {},
): SignalingSocketClient {
	return createSocketClient<SignalingEventType, SignalingMessage>(
		url,
		handlers,
	);
}

function createSocketClient<TEventType extends string, TMessage>(
	url: string,
	handlers: SocketHandlers<TMessage> = {},
) {
	// ブラウザ標準の WebSocket を、アプリで使いやすい形に薄く包む。
	// ここに JSON parse/stringify と接続状態の通知を閉じ込めている。
	const socket = new WebSocket(url);
	let currentState: WorldSocketState = "connecting";

	function notifyStateChange(newState: typeof currentState) {
		// 同じ状態を何度も通知しないようにして、UI の無駄な再描画を減らす。
		if (currentState !== newState) {
			currentState = newState;
			handlers.onStateChange?.(newState);
		}
	}

	notifyStateChange("connecting");

	socket.addEventListener("open", () => {
		notifyStateChange("connected");
		handlers.open?.();
	});

	socket.addEventListener("message", (event) => {
		// このアプリの protocol は JSON 文字列前提。
		// Blob や ArrayBuffer が来た場合は扱わずに捨てる。
		if (typeof event.data !== "string") {
			return;
		}

		try {
			handlers.message?.(JSON.parse(event.data) as TMessage);
		} catch (error) {
			console.warn("Failed to parse websocket message", error);
		}
	});

	socket.addEventListener("close", (event) => {
		notifyStateChange("disconnected");
		handlers.close?.(event);
	});

	socket.addEventListener("error", (event) => {
		notifyStateChange("error");
		handlers.error?.(event);
	});

	function send<TPayload>(type: TEventType, payload: TPayload) {
		// 未接続の時に send() すると例外になるため、開いている時だけ送る。
		// 呼び出し側は毎回 readyState を気にしなくてよい。
		if (socket.readyState !== WebSocket.OPEN) {
			return;
		}

		const message: SocketMessage<TEventType, TPayload> = {
			t: type,
			d: payload,
		};

		socket.send(JSON.stringify(message));
	}

	function close() {
		socket.close();
	}

	function isOpen() {
		return socket.readyState === WebSocket.OPEN;
	}

	function getState() {
		return currentState;
	}

	return { send, close, isOpen, getState };
}
