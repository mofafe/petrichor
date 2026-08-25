import { normalizeRoomID, roomIDFromLocation } from "../net/websocket";
import { PLAYER_NAME_STORAGE_KEY, ROOM_PATH } from "./constants";
import { onEnter, queryRequired } from "./dom";

// 最初の画面を動かす関数。
// ここでは 3D や WebSocket はまだ起動せず、名前とルーム ID を決めるだけにしている。
export function startEntryView() {
	const nameInput = queryRequired<HTMLInputElement>("#entry-name");
	const roomInput = queryRequired<HTMLInputElement>("#entry-room");
	const joinButton = queryRequired<HTMLButtonElement>("#entry-join");

	// 前回使った名前があれば復元する。
	// ルーム ID は URL の `?room=` があればそれを使い、なければ websocket.ts 側の既定値になる。
	nameInput.value = localStorage.getItem(PLAYER_NAME_STORAGE_KEY) || "guest";
	roomInput.value = roomIDFromLocation();

	// 入室時はフォームの値を整えてから `/r?room=...` に移動する。
	// 実際のルーム起動は移動後に main.ts -> startRoomView() で行われる。
	function joinRoom() {
		const name = nameInput.value.trim() || "guest";
		const roomID = normalizeRoomID(roomInput.value);
		const url = new URL(window.location.href);

		localStorage.setItem(PLAYER_NAME_STORAGE_KEY, name);
		url.pathname = ROOM_PATH;
		url.search = "";
		url.searchParams.set("room", roomID);
		window.location.href = url.toString();
	}

	// クリックでも Enter でも同じ入室処理を使う。
	// 処理を 1 つに寄せると、後でバリデーションを足す時も joinRoom() だけを見ればよい。
	joinButton.addEventListener("click", joinRoom);
	onEnter(roomInput, joinRoom);
	onEnter(nameInput, joinRoom);
}
