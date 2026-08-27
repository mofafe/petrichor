import { Clock } from "three";
import { createPlayer } from "../core/player";
import { createInput } from "../input/input";
import {
	createWorldSocket,
	createSignalingSocket,
	roomIDFromLocation,
	signalingWebSocketURL,
	worldWebSocketURL,
	type JoinPayload,
	type MovePayload,
	type PlayerStatePayload,
	type SignalingMessage,
	type SignalingSocketClient,
	type StateSyncPayload,
	type WorldMessage,
} from "../net/websocket";
import { createScene } from "../scene/scene";
import { createVoiceChat } from "../voice/voice";
import { createConnectionStatusView } from "./connectionStatus";
import { PLAYER_NAME_STORAGE_KEY, ROOM_PATH } from "./constants";
import { queryRequired } from "./dom";

const MOVE_SEND_INTERVAL_MS = 80;
const RENDER_SCALE = 0.35;
const INVITE_FEEDBACK_MS = 1600;

// 3D ルーム画面を起動する関数。
// ここは「部品をつなぐ場所」で、描画は scene.ts、入力は input.ts、移動は player.ts、音声は voice.ts に任せる。
export function startRoomView(canvas: HTMLCanvasElement) {
	// URL から room ID を先に決める。
	// 名前が未保存の場合でも、フォーム入力後に同じ room へ入れるように保持しておく。
	const roomID = roomIDFromLocation();
	const storedPlayerName = localStorage
		.getItem(PLAYER_NAME_STORAGE_KEY)
		?.trim();

	if (!storedPlayerName) {
		// Invite URL などから直接 `/r?room=...` を開いた場合は、名前だけここで入力してもらう。
		// 名前が決まるまで WebSocket を開かないので、guest で join してしまうことを避けられる。
		showRoomNamePrompt((playerName) => {
			startRoomSession(canvas, roomID, playerName);
		});
		return;
	}

	startRoomSession(canvas, roomID, storedPlayerName);
}

function startRoomSession(
	canvas: HTMLCanvasElement,
	roomID: string,
	playerName: string,
) {
	// ルームに必要な主要部品を最初に作る。
	// world は Three.js の世界、input は現在のキー/マウス状態、player は自分の移動計算を担当する。
	const world = createScene(canvas);
	const input = createInput(canvas);
	const player = createPlayer(world.camera, input);
	const clock = new Clock();

	// HTML 上にある操作 UI を取得する。
	// queryRequired() を使うと、HTML 側の id/class 名が変わった時に早めにエラーで気づける。
	const micButton = queryRequired<HTMLButtonElement>("#mic-toggle");
	const inviteButton = queryRequired<HTMLButtonElement>(".invite");
	const voiceStatus = queryRequired<HTMLElement>("#voice-status");
	const statusDot = queryRequired<HTMLElement>(".status-dot");

	// WebSocket と WebRTC とマイクの状態を、画面上の 1 つのステータス表示へまとめる。
	const connectionStatus = createConnectionStatusView(
		voiceStatus,
		statusDot,
	);

	// 他プレイヤーの最新状態をクライアント側でも保持する。
	// move だけでは名前や speaking が来ないため、前回状態と合成する時に使う。
	const remotePlayers = new Map<string, PlayerStatePayload>();

	// localUserID はサーバーから最初に返ってきた自分の user ID。
	// 自分自身を remote player として描画しないために使う。
	let localUserID: string | undefined;
	let lastMoveSentAt = 0;
	let micEnabled = false;
	let signalingSocket: SignalingSocketClient | undefined;

	// world 用 WebSocket を開く。
	// open 時に join を送ることで、サーバーへ「このルームにこの名前/位置で参加した」と伝える。
	const worldSocket = createWorldSocket(worldWebSocketURL(roomID), {
		open() {
			worldSocket.send<JoinPayload>("join", {
				name: playerName,
				...player.statePayload(),
			});
		},
		message(message) {
			// voice.handleMessage() が非同期なので、イベントリスナー内では promise を待たずに流す。
			// 実際の処理は handleWorldMessage() の中で await して順番に見る。
			void handleWorldMessage(message);
		},
		close(event) {
			console.info("world websocket closed", event.code, event.reason);
		},
		error(event) {
			console.warn("world websocket error", event);
		},
		onStateChange(state) {
			connectionStatus.setWebSocketState(state);
		},
	});

	// 音声通話の部品を作る。
	// 音声そのものは WebRTC、offer/answer/candidate の交換は signaling socket を使う。
	const voice = createVoiceChat({
		worldSocket,
		onStatus(status) {
			connectionStatus.setVoiceStatus(status);
		},
		onPeerStateChange(state) {
			connectionStatus.setPeerState(state);
		},
	});

	// サーバーから来た world message を種類ごとに分ける。
	async function handleWorldMessage(message: WorldMessage) {
		switch (message.t) {
			case "join":
				handleJoinMessage(message);
				return;
			case "move":
				handleMoveMessage(message);
				return;
			case "state_sync":
				syncRemotePlayers((message.d as StateSyncPayload).players);
				return;
			case "speak_start":
			case "speak_stop":
				handleSpeakMessage(message);
				return;
			case "leave":
				handleLeaveMessage(message);
				return;
			case "room_full":
				showRoomFullNotice();
				worldSocket.close();
				return;
			default:
				return;
		}
	}

	async function handleSignalingMessage(message: SignalingMessage) {
		await voice.handleMessage(message);
	}

	// join は 2 通りの意味を持つ。
	// 最初の join は「自分の user ID が決まった」通知、それ以降は「他人が入ってきた」通知として扱う。
	function handleJoinMessage(message: WorldMessage) {
		if (!message.u) {
			return;
		}

		if (!localUserID) {
			localUserID = message.u;
			voice.setLocalUserID(localUserID);
			openSignalingSocket(localUserID);
			world.removeRemotePlayer(localUserID);
			voice.removeRemotePlayer(localUserID);
			return;
		}

		upsertRemotePlayer({
			...(message.d as JoinPayload),
			u: message.u,
			speaking: false,
		});
	}

	// move は他プレイヤーの位置と回転だけを更新する。
	// 名前や speaking は move payload に入っていないため、保存済みの状態から引き継ぐ。
	function handleMoveMessage(message: WorldMessage) {
		if (!message.u) {
			return;
		}

		const current = remotePlayers.get(message.u);
		upsertRemotePlayer({
			...(message.d as MovePayload),
			u: message.u,
			name: current?.name ?? "",
			speaking: current?.speaking ?? false,
		});
	}

	// speak_start / speak_stop は見た目と音声側の状態を更新する。
	// 位置は変わらないため、現在の player state に speaking だけ上書きする。
	function handleSpeakMessage(message: WorldMessage) {
		if (!message.u) {
			return;
		}

		const current = remotePlayers.get(message.u);
		if (!current) {
			return;
		}

		upsertRemotePlayer({
			...current,
			speaking: message.t === "speak_start",
		});
	}

	// leave が来たら、見た目・音声・保存中の状態を全部消す。
	// どれか 1 つだけ残ると、画面に残像が出たり音声 peer が残ったりする。
	function handleLeaveMessage(message: WorldMessage) {
		if (!message.u) {
			return;
		}

		world.removeRemotePlayer(message.u);
		voice.removeRemotePlayer(message.u);
		remotePlayers.delete(message.u);
	}

	// 他プレイヤー 1 人分の状態を、保存・描画・音声の 3 か所へ反映する。
	// 自分の ID と同じ場合は remote として扱わない。
	function upsertRemotePlayer(playerState: PlayerStatePayload) {
		if (playerState.u === localUserID) {
			return;
		}

		remotePlayers.set(playerState.u, playerState);
		world.upsertRemotePlayer(playerState, localUserID);
		voice.upsertRemotePlayer(playerState);
	}

	// state_sync はサーバーが持っている現在の参加者一覧。
	// 一覧にいない player は退出済みとして削除し、一覧にいる player は最新状態へ揃える。
	function syncRemotePlayers(players: PlayerStatePayload[]) {
		const seen = new Set<string>();
		const remoteStates: PlayerStatePayload[] = [];

		for (const playerState of players) {
			if (playerState.u === localUserID) {
				continue;
			}

			seen.add(playerState.u);
			remoteStates.push(playerState);
			remotePlayers.set(playerState.u, playerState);
		}

		for (const id of remotePlayers.keys()) {
			if (!seen.has(id)) {
				remotePlayers.delete(id);
			}
		}

		world.syncRemotePlayers(remoteStates, localUserID);
		voice.syncRemotePlayers(remoteStates);
	}

	// 画面サイズが変わった時に、カメラの縦横比と renderer の描画サイズを合わせる。
	// renderer は低めの解像度にして、CSS で引き伸ばすことでドット感を出している。
	function resize() {
		const width = window.innerWidth;
		const height = window.innerHeight;

		world.camera.aspect = width / height;
		world.camera.updateProjectionMatrix();
		world.renderer.setPixelRatio(1);
		world.renderer.setSize(
			Math.floor(width * RENDER_SCALE),
			Math.floor(height * RENDER_SCALE),
			false,
		);
	}

	// 毎フレーム呼ばれるメインループ。
	// 入力 -> 自分の移動 -> 音声位置 -> 必要なら move 送信 -> 描画、の順に進む。
	function animate() {
		// delta は前フレームからの経過秒数。
		// 上限を付けて、タブ復帰直後などに一気に大きく動かないようにしている。
		const delta = Math.min(clock.getDelta(), 0.05);

		player.update(delta);
		const playerState = player.statePayload();
		voice.updateLocalPosition(playerState.x, playerState.y);
		sendMoveIfNeeded();
		world.update();
		world.renderer.render(world.scene, world.camera);
		requestAnimationFrame(animate);
	}

	// 自分の位置をサーバーへ送るか決める。
	// WebSocket が開いていて、前回送信から少し時間が経ち、実際に動いた時だけ送る。
	function sendMoveIfNeeded() {
		const now = performance.now();

		if (
			!worldSocket.isOpen() ||
			now - lastMoveSentAt < MOVE_SEND_INTERVAL_MS ||
			!player.consumeMoved()
		) {
			return;
		}

		lastMoveSentAt = now;
		worldSocket.send<MovePayload>("move", player.statePayload());
	}

	// 退出時の共通処理。
	// ボタン退出とページ閉じる時の両方から呼び、音声停止と WebSocket close を揃える。
	function leaveRoom() {
		voice.disable();
		worldSocket.send("leave", {});
		worldSocket.close();
		signalingSocket?.close();
	}

	function openSignalingSocket(userID: string) {
		if (signalingSocket) {
			return;
		}

		signalingSocket = createSignalingSocket(
			signalingWebSocketURL(roomID, userID),
			{
				open() {
					voice.flushSignalingMessages();
				},
				message(message) {
					void handleSignalingMessage(message);
				},
				close(event) {
					console.info(
						"signaling websocket closed",
						event.code,
						event.reason,
					);
				},
				error(event) {
					console.warn("signaling websocket error", event);
				},
			},
		);
		voice.setSignalingSocket(signalingSocket);
	}

	function inviteURL() {
		// 今いる room ID を含む共有用 URL を作る。
		// hash や余計な query は持ち越さず、入室に必要な `?room=` だけにする。
		const url = new URL(window.location.href);
		url.pathname = ROOM_PATH;
		url.search = "";
		url.hash = "";
		url.searchParams.set("room", roomID);
		return url.toString();
	}

	async function copyInviteURL() {
		const text = inviteURL();

		// Clipboard API は HTTPS や localhost などの secure context で使える。
		// 使えない環境では textarea を一時的に作る古い方法へ fallback する。
		if (navigator.clipboard?.writeText) {
			await navigator.clipboard.writeText(text);
			return;
		}

		const textarea = document.createElement("textarea");
		textarea.value = text;
		textarea.readOnly = true;
		textarea.style.position = "fixed";
		textarea.style.left = "-9999px";
		document.body.append(textarea);
		textarea.select();

		try {
			const copied = document.execCommand("copy");
			if (!copied) {
				throw new Error("Copy command was rejected");
			}
		} finally {
			textarea.remove();
		}
	}

	function showInviteButtonText(text: string) {
		inviteButton.textContent = text;
		window.setTimeout(() => {
			inviteButton.textContent = "Invite";
		}, INVITE_FEEDBACK_MS);
	}

	// ブラウザを閉じる/更新する直前に、できる範囲で leave を送る。
	window.addEventListener("beforeunload", leaveRoom);
	window.addEventListener("pagehide", leaveRoom);
	document.addEventListener("visibilitychange", reloadRoomIfSocketClosed);
	document
		.querySelector<HTMLButtonElement>(".leave")
		?.addEventListener("click", () => {
			// Leave ボタンでは明示的にトップへ戻る。
			// beforeunload とは違い、ユーザーの操作として画面遷移まで行う。
			leaveRoom();
			window.location.href = "/iolite";
		});

	inviteButton.addEventListener("click", () => {
		copyInviteURL()
			.then(() => {
				showInviteButtonText("Copied");
			})
			.catch((error) => {
				showInviteButtonText("Copy failed");
				console.warn("invite copy failed", error);
			});
	});

	// マイクボタンは ON/OFF の切り替え。
	// enable() はブラウザのマイク許可ダイアログを出す可能性があるため Promise で扱う。
	micButton.addEventListener("click", () => {
		if (micEnabled) {
			voice.disable();
			micEnabled = false;
			micButton.textContent = "Mic Off";
			return;
		}

		voice
			.enable()
			.then(() => {
				micEnabled = true;
				micButton.textContent = "Mic On";
			})
			.catch((error) => {
				voiceStatus.textContent = "mic error";
				console.warn("microphone start failed", error);
			});
	});

	window.addEventListener("resize", resize);
	resize();
	animate();

	function reloadRoomIfSocketClosed() {
		if (document.visibilityState !== "visible" || worldSocket.isOpen()) {
			return;
		}

		window.location.reload();
	}
}

function showRoomNamePrompt(onSubmit: (playerName: string) => void) {
	const prompt = queryRequired<HTMLElement>("#room-name-prompt");
	const form = queryRequired<HTMLFormElement>("#room-name-form");
	const nameInput = queryRequired<HTMLInputElement>("#room-name-input");

	prompt.hidden = false;
	nameInput.value = "";
	nameInput.focus();

	form.addEventListener(
		"submit",
		(event) => {
			event.preventDefault();

			const playerName = nameInput.value.trim();
			if (!playerName) {
				nameInput.focus();
				return;
			}

			localStorage.setItem(PLAYER_NAME_STORAGE_KEY, playerName);
			prompt.hidden = true;
			onSubmit(playerName);
		},
		{ once: true },
	);
}

// ルーム人数上限に達した時の通知を表示する。
// 通知自体は HTML に置いてあり、ここでは hidden を外すだけ。
function showRoomFullNotice() {
	const notice = document.getElementById("room-full");
	if (notice) {
		notice.hidden = false;
	}
}
