export type InputState = {
	// 現在押されているキーを入れておく Set。
	// `KeyW` などの KeyboardEvent.code を入れるので、player.ts はここだけ見れば移動方向を決められる。
	keys: Set<string>;
	// マウス操作が canvas に固定されているかどうか。
	// 固定されていない時は、普通のページ操作としてマウスを扱う。
	pointerLocked: boolean;
	// 前回のフレームから今回までにマウスがどれだけ動いたか。
	// player.ts が毎フレーム consumeLookDelta() で読み取り、読み取ったら 0 に戻す。
	lookDeltaX: number;
	lookDeltaY: number;
	consumeLookDelta: () => { x: number; y: number };
};

// 入力イベントを 1 か所に集める関数。
// このファイルは「キーやマウスの状態を記録するだけ」で、プレイヤーを動かす計算は player.ts が行う。
export function createInput(canvas: HTMLCanvasElement): InputState {
	const input: InputState = {
		keys: new Set<string>(),
		pointerLocked: false,
		lookDeltaX: 0,
		lookDeltaY: 0,
		consumeLookDelta() {
			// マウス移動量は「今回のフレームで使う分」だけ返す。
			// 返した後に 0 に戻さないと、同じマウス移動が次のフレームでも残ってしまう。
			const delta = { x: this.lookDeltaX, y: this.lookDeltaY };
			this.lookDeltaX = 0;
			this.lookDeltaY = 0;
			return delta;
		},
	};

	// キーを押したら Set に追加する。
	// Set は同じキーを何度 add しても 1 つだけなので、押しっぱなしの管理に向いている。
	window.addEventListener("keydown", (event) => {
		input.keys.add(event.code);
	});

	// キーを離したら Set から消す。
	// player.ts は Set に残っているキーだけを見て移動するため、消えたキーの方向には進まなくなる。
	window.addEventListener("keyup", (event) => {
		input.keys.delete(event.code);
	});

	// canvas をクリックしたら Pointer Lock を要求する。
	// Pointer Lock 中はマウスカーソルが画面端で止まらず、FPS 視点のように見回せる。
	canvas.addEventListener("click", () => {
		canvas.requestPointerLock();
	});

	// Pointer Lock の状態はブラウザ側で変わる。
	// Esc キーで解除された時もここが呼ばれるため、現在の状態を同期しておく。
	document.addEventListener("pointerlockchange", () => {
		input.pointerLocked = document.pointerLockElement === canvas;
	});

	// マウス移動量を足し込む。
	// pointerLocked でない時は、ページ上を普通に動かしているだけなので視点操作には使わない。
	window.addEventListener("mousemove", (event) => {
		if (!input.pointerLocked) {
			return;
		}

		input.lookDeltaX += event.movementX;
		input.lookDeltaY += event.movementY;
	});

	return input;
}
