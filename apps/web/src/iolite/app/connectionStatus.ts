import {
	VOICE_STATUS,
	type VoicePeerState,
	type VoiceStatus,
} from "../voice/voice";
import type { WorldSocketState } from "../net/websocket";

const STATUS_DOT_CLASSES = [
	"status-error",
	"status-disconnected",
	"status-connecting",
	"status-connected",
	"status-failed",
];

// 表示する文字と、色を変えるための CSS クラスをまとめた型。
// DOM 更新の前にこの形へ変換しておくと、判定ロジックと画面反映を分けて読める。
type StatusView = {
	text: string;
	className: string;
};

// WebSocket、WebRTC、マイク状態を 1 つの表示にまとめる UI 部品。
// room.ts は状態が変わった時に setter を呼ぶだけで、細かい文言の分岐を知らなくてよい。
export function createConnectionStatusView(
	voiceStatus: HTMLElement,
	statusDot: HTMLElement,
) {
	// 3 種類の状態をこの関数内に保持する。
	// どれか 1 つが変わるたびに render() し、今の組み合わせに合う表示へ更新する。
	let wsState: WorldSocketState = "connecting";
	let peerState: VoicePeerState = "idle";
	let voiceStatusText: VoiceStatus = VOICE_STATUS.MIC_OFF;

	function render() {
		const view = statusView(wsState, peerState, voiceStatusText);

		// テキストを更新し、ステータス丸の色用クラスを付け替える。
		// 古いクラスを先に全部消すことで、前の状態の色が残らないようにしている。
		voiceStatus.textContent = view.text;
		statusDot.classList.remove(...STATUS_DOT_CLASSES);
		if (view.className) {
			statusDot.classList.add(view.className);
		}
	}

	return {
		setWebSocketState(state: WorldSocketState) {
			wsState = state;
			render();
		},
		setPeerState(state: VoicePeerState) {
			peerState = state;
			render();
		},
		setVoiceStatus(status: VoiceStatus) {
			voiceStatusText = status;
			render();
		},
	};
}

// 状態の優先順位を決める関数。
// マイクエラーやサーバーエラーのようにユーザーが知るべきものを先に判定する。
function statusView(
	wsState: WorldSocketState,
	peerState: VoicePeerState,
	voiceStatusText: VoiceStatus,
): StatusView {
	if (voiceStatusText === VOICE_STATUS.MIC_ERROR) {
		return { text: "マイクエラー", className: "status-error" };
	}

	if (wsState === "error") {
		return { text: "サーバー接続エラー", className: "status-error" };
	}

	if (wsState === "disconnected") {
		return {
			text: "未接続 (再読込してください)",
			className: "status-disconnected",
		};
	}

	if (wsState === "connecting") {
		return { text: "接続中...", className: "status-connecting" };
	}

	if (peerState === "failed") {
		return { text: "通話ルート確認失敗", className: "status-failed" };
	}

	if (peerState === "connecting") {
		return {
			text: "接続済み・通話準備中...",
			className: "status-connecting",
		};
	}

	if (peerState === "connected") {
		return { text: "接続済み・通話可能", className: "status-connected" };
	}

	if (isReadableMicStatus(voiceStatusText)) {
		return {
			text: translateVoiceStatus(voiceStatusText),
			className: "status-connected",
		};
	}

	return { text: "接続済み・相手待ち", className: "status-connected" };
}

// そのままユーザーへ見せても分かりやすいマイク状態だけを通す。
// ICE 取得中などは、接続済み・相手待ちの方が全体状態として自然な場面がある。
function isReadableMicStatus(status: VoiceStatus) {
	return (
		status === VOICE_STATUS.MIC_ON ||
		status === VOICE_STATUS.MIC_OFF ||
		status === VOICE_STATUS.TAP_MIC_TO_START_AUDIO
	);
}

// voice.ts 内部の英語ステータスを UI 表示用の日本語へ変換する。
// 内部値を日本語にしてしまうと、条件分岐やログで扱いにくくなるため表示直前で変える。
function translateVoiceStatus(status: VoiceStatus): string {
	switch (status) {
		case VOICE_STATUS.ICE_REQUESTING:
			return "通話ルート確認中...";
		case VOICE_STATUS.MIC_REQUESTING:
			return "マイク許可待ち...";
		case VOICE_STATUS.MIC_ON:
			return "マイク ON";
		case VOICE_STATUS.MIC_OFF:
			return "マイク OFF";
		case VOICE_STATUS.MIC_ERROR:
			return "マイクエラー";
		case VOICE_STATUS.TAP_MIC_TO_START_AUDIO:
			return "タップして音声を開始";
		default:
			console.warn("Unknown voice status:", status);
			return status;
	}
}
