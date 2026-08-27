import { Euler, MathUtils, PerspectiveCamera, Vector3 } from "three";
import type { InputState } from "../input/input";
import type { MovePayload, RotationPayload } from "../net/websocket";

// 毎フレーム作り直すと小さなメモリ確保が増えるため、
// 移動計算に使うベクトルはファイルの上で使い回す。
const forward = new Vector3();
const right = new Vector3();
const move = new Vector3();

// プレイヤー操作を作る関数。
// このアプリでは Three.js の camera 自体を「自分の目」として動かしている。
export function createPlayer(camera: PerspectiveCamera, input: InputState) {
	// Euler は「縦に向く角度」「横に向く角度」を扱うための回転表現。
	// YXZ の順にして、FPS 視点のように左右回転と上下回転を自然に組み合わせる。
	const rotation = new Euler(0, 0, 0, "YXZ");
	const eyeHeight = 1.45;
	const speed = 4.2;
	const lookSensitivity = 0.0024;
	// true の時だけ WebSocket に move を送る。
	// 動いていないフレームまで毎回送ると通信量が増えるため、変更があったかを覚えておく。
	let moved = true;

	// カメラ自体をプレイヤーの目として使う。
	// Three.js ではカメラの位置と向きが、そのままプレイヤーの見ている場所になる。
	camera.position.set(0, eyeHeight, 0);
	rotation.set(0, Math.PI, 0);

	function update(delta: number) {
		// マウス移動量を回転角度に変換する。
		// 毎フレーム消費することで、同じマウス入力を何度も使わないようにしている。
		const look = input.consumeLookDelta();
		rotation.y -= look.x * lookSensitivity;
		rotation.x -= look.y * lookSensitivity;
		if (look.x !== 0 || look.y !== 0) {
			moved = true;
		}

		// 上下の見上げ・見下ろし角を制限する。
		// 制限しないと真上や真下を超えて、視点が反転したように感じるため。
		rotation.x = MathUtils.clamp(rotation.x, -1.15, 1.15);
		camera.quaternion.setFromEuler(rotation);

		// カメラが向いている方向から、地面に沿った前方向ベクトルを作る。
		// y を 0 にするのは、上を向いても空へ進まず水平移動にするため。
		forward.set(0, 0, -1).applyQuaternion(camera.quaternion);
		forward.y = 0;
		forward.normalize();

		// カメラの右方向も同じように地面に沿わせる。
		// A/D キーで横移動するために、前方向とは別に右方向が必要。
		right.set(1, 0, 0).applyQuaternion(camera.quaternion);
		right.y = 0;
		right.normalize();

		// 今フレームの移動量を毎回リセットする。
		// 押されているキーだけを元に計算し直すことで、キーを離したら止まる。
		move.set(0, 0, 0);

		if (input.keys.has("KeyW")) move.add(forward);
		if (input.keys.has("KeyS")) move.sub(forward);
		if (input.keys.has("KeyD")) move.add(right);
		if (input.keys.has("KeyA")) move.sub(right);

		// 斜め移動が速くなりすぎないよう正規化してから、速度と delta を掛ける。
		// delta を使うことで、フレームレートが違っても移動距離をそろえられる。
		if (move.lengthSq() > 0) {
			move.normalize().multiplyScalar(speed * delta);
			camera.position.add(move);
			moved = true;
		}

		// カメラの高さを目線の高さに固定しつつ、少しだけ揺らす。
		// 完全に静止した視点より、歩いているような感覚を出すため。
		// camera.position.y = eyeHeight + Math.sin(performance.now() * 0.008) * 0.015;
	}

	function rotationPayload(): RotationPayload {
		// サーバーや他プレイヤーへ送るため、Three.js の Euler から plain object に変換する。
		// class インスタンスをそのまま送るより、JSON として扱いやすい形になる。
		return {
			x: rotation.x,
			y: rotation.y,
			z: rotation.z,
		};
	}

	function statePayload(): MovePayload {
		// 3D 空間では x/z が床の平面座標。
		// 通信 payload では `y` に z 座標を入れて、2D 的な `{ x, y }` として扱っている。
		return {
			x: camera.position.x,
			y: camera.position.z,
			rotation: rotationPayload(),
		};
	}

	function consumeMoved() {
		// 「動いたか」を 1 回だけ取り出す。
		// room.ts はこれを見て move を送信し、ここで false に戻す。
		const wasMoved = moved;
		moved = false;
		return wasMoved;
	}

	return { update, statePayload, consumeMoved };
}
