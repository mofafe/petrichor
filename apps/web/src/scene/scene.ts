import {
	AmbientLight,
	BoxGeometry,
	Group,
	Color,
	DirectionalLight,
	Fog,
	GridHelper,
	Mesh,
	MeshBasicMaterial,
	MeshLambertMaterial,
	NearestFilter,
	PerspectiveCamera,
	PlaneGeometry,
	Scene,
	Sprite,
	SpriteMaterial,
	CanvasTexture,
	WebGLRenderer,
} from "three";
import type { PlayerStatePayload } from "../net/websocket";

const floorSize = 42;

// 画面に表示する他プレイヤー 1 人分の Three.js オブジェクト。
// group を動かすと body と facing が一緒に動く。
type RemotePlayer = {
	group: Group;
	avatar: Group;
	body: Mesh;
	facing: Mesh;
	name: string;
	nameLabel: Sprite;
};

// 床に貼る小さなドット絵風テクスチャを Canvas で作る。
// 画像ファイルを用意しなくても、Three.js のマテリアルに渡せる模様を生成できるため。
function makePixelTexture(): CanvasTexture {
	const size = floorSize;
	const canvas = document.createElement("canvas");
	canvas.width = size;
	canvas.height = size;

	const ctx = canvas.getContext("2d");
	if (!ctx) {
		throw new Error("Could not create pixel texture");
	}

	//ctx.fillStyle = "#121212";
	ctx.fillStyle = "#00ff00";
	ctx.fillRect(0, 0, size, size);

	/*for (let y = 0; y < size; y += 1) {
		for (let x = 0; x < size; x += 1) {
			ctx.fillStyle = (x + y) % 2 === 0 ? "#101010" : "#080808";
			ctx.fillRect(x, y, 1, 1);
		}
	}*/

	// CanvasTexture にすると、Canvas に描いた内容を Three.js の texture として使える。
	// NearestFilter は拡大時にぼかさず、ピクセルの角を残すために必要。
	const texture = new CanvasTexture(canvas);
	texture.magFilter = NearestFilter;
	texture.minFilter = NearestFilter;
	return texture;
}

function makeNameLabelTexture(name: string): CanvasTexture {
	const canvas = document.createElement("canvas");
	canvas.width = 256;
	canvas.height = 64;

	const ctx = canvas.getContext("2d");
	if (!ctx) {
		throw new Error("Could not create name label texture");
	}

	ctx.clearRect(0, 0, canvas.width, canvas.height);
	ctx.fillStyle = "rgba(5, 6, 8, 0.82)";
	ctx.fillRect(0, 8, canvas.width, 42);
	ctx.strokeStyle = "#2ff0ad";
	ctx.lineWidth = 3;
	ctx.strokeRect(1.5, 9.5, canvas.width - 3, 39);
	ctx.fillStyle = "#f8f7dc";
	ctx.font = "24px 'Courier New', monospace";
	ctx.textAlign = "center";
	ctx.textBaseline = "middle";
	ctx.fillText(name || "guest", canvas.width / 2, 31, canvas.width - 24);

	const texture = new CanvasTexture(canvas);
	texture.magFilter = NearestFilter;
	texture.minFilter = NearestFilter;
	return texture;
}

function createNameLabel(name: string): Sprite {
	const material = new SpriteMaterial({
		map: makeNameLabelTexture(name),
		transparent: true,
		depthTest: false,
		depthWrite: false,
	});
	const label = new Sprite(material);
	label.position.y = 1.65;
	label.scale.set(1.8, 0.45, 1);
	label.renderOrder = 10;
	return label;
}

function setNameLabel(label: Sprite, name: string) {
	const material = label.material as SpriteMaterial;
	material.map?.dispose();
	material.map = makeNameLabelTexture(name);
	material.needsUpdate = true;
}

export function createScene(canvas: HTMLCanvasElement) {
	// Scene は Three.js の世界そのもの。
	// Mesh、Light、Camera など、描画したいものはこの中に追加していく。
	const scene = new Scene();
	scene.background = new Color("#90d7ec");
	scene.fog = new Fog("#a0d8e0", 4, 20);

	// user ID -> 画面上の 3D オブジェクト。
	// 通信で状態が来るたびに、この Map から該当プレイヤーの Mesh を探して更新する。
	const remotePlayers = new Map<string, RemotePlayer>();

	// 他プレイヤーの見た目に使うマテリアル。
	// speaking の時だけ body のマテリアルを変えて、話していることを見た目でも分かるようにする。
	const remoteBodyMaterial = new MeshLambertMaterial({ color: "#6be7c7" });
	const speakingBodyMaterial = new MeshLambertMaterial({ color: "#f5ffd7" });
	const remoteFacingMaterial = new MeshBasicMaterial({ color: "#ff4f7a" });

	// PerspectiveCamera は人間の目に近い遠近感を持つカメラ。
	// 視野角、画面の縦横比、近くと遠くの描画範囲を指定して、何を画面に映すか決める。
	const camera = new PerspectiveCamera(
		68,
		window.innerWidth / window.innerHeight,
		0.1,
		60,
	);

	// WebGLRenderer は Scene と Camera の情報を使って canvas に描画する担当。
	// antialias を false にして、なめらかにしすぎない低解像度風の見た目にしている。
	const renderer = new WebGLRenderer({
		canvas,
		antialias: false,
		powerPreference: "high-performance",
	});

	renderer.setClearColor("#07080a");

	// 床は PlaneGeometry と MeshLambertMaterial を組み合わせた Mesh。
	// Geometry が形、Material が見た目、Mesh がそれらをシーンに置ける物体にする役割。
	const floor = new Mesh(
		new PlaneGeometry(floorSize, floorSize, floorSize, floorSize),
		new MeshLambertMaterial({
			color: "#ffffff",
			map: makePixelTexture(),
		}),
	);
	floor.rotation.x = -Math.PI / 2;
	scene.add(floor);

	// GridHelper は床の上に目安線を出す補助オブジェクト。
	// 3D 空間では距離や向きが分かりにくいので、移動感をつかむために置いている。
	// const grid = new GridHelper(floorSize, floorSize, "#75ffd2", "#5f5f5f");
	const grid = new GridHelper(floorSize, floorSize, "#75ffd2", "#00582d");
	grid.position.y = 0.01;
	scene.add(grid);

	// AmbientLight はシーン全体を薄く照らすライト。
	// これがないと、直接光が当たらない面が真っ黒になりすぎる。
	const ambient = new AmbientLight("#6c8f99", 1.2);
	scene.add(ambient);

	// DirectionalLight は太陽のように一方向から照らすライト。
	// MeshLambertMaterial の明暗が出るので、床や物体の立体感を作るために必要。
	const sun = new DirectionalLight("#f5ffd7", 1.8);
	sun.position.set(4, 8, 5);
	scene.add(sun);

	function createRemotePlayer(): RemotePlayer {
		// Group は複数の Mesh をまとめて動かすための親。
		// 体と向きマーカーを別 Mesh にしておくと、後で見た目を足しやすい。
		const group = new Group();
		const avatar = new Group();
		group.add(avatar);

		const body = new Mesh(
			new BoxGeometry(0.45, 1.2, 0.45),
			remoteBodyMaterial,
		);
		body.position.y = 0.65;
		avatar.add(body);

		const facing = new Mesh(
			new BoxGeometry(0.18, 0.18, 0.12),
			remoteFacingMaterial,
		);
		facing.position.set(0, 1.05, -0.28);
		avatar.add(facing);

		const nameLabel = createNameLabel("");
		group.add(nameLabel);

		scene.add(group);
		return { group, avatar, body, facing, name: "", nameLabel };
	}

	function removeRemote(remote: RemotePlayer) {
		const material = remote.nameLabel.material as SpriteMaterial;
		material.map?.dispose();
		material.dispose();
		scene.remove(remote.group);
	}

	// 他プレイヤーを新規作成または更新する。
	// room.ts は通信 payload を渡すだけで、具体的な Mesh の操作はここに閉じ込める。
	function upsertRemotePlayer(
		player: PlayerStatePayload,
		localUserID?: string,
	) {
		// 自分自身の状態が state_sync に含まれていても、remote player としては描かない。
		if (player.u === localUserID) {
			return;
		}

		// まだ画面にいない player は createRemotePlayer() で作り、既にいれば再利用する。
		const remote = remotePlayers.get(player.u) ?? createRemotePlayer();
		remotePlayers.set(player.u, remote);

		// 通信上の y は Three.js の z として扱う。
		// Three.js の y は高さ方向なので、床の上では 0 に固定する。
		remote.group.position.set(player.x, 0, player.y);
		remote.avatar.rotation.set(
			player.rotation.x,
			player.rotation.y,
			player.rotation.z,
		);
		if (remote.name !== player.name) {
			remote.name = player.name;
			setNameLabel(remote.nameLabel, player.name);
		}
		remote.body.material = player.speaking
			? speakingBodyMaterial
			: remoteBodyMaterial;
		remote.facing.scale.setScalar(player.speaking ? 1.45 : 1);
	}

	// サーバーから届いた全 player 一覧に合わせて画面を同期する。
	// 一覧に存在しない player は退出済みとして Mesh を削除する。
	function syncRemotePlayers(
		players: PlayerStatePayload[],
		localUserID?: string,
	) {
		const seen = new Set<string>();

		for (const player of players) {
			if (player.u === localUserID) {
				continue;
			}

			seen.add(player.u);
			upsertRemotePlayer(player, localUserID);
		}

		for (const [id, remote] of remotePlayers) {
			if (!seen.has(id)) {
				removeRemote(remote);
				remotePlayers.delete(id);
			}
		}
	}

	// 指定した user ID の remote player を画面から消す。
	// leave 受信時や、自分自身を remote として消したい時に使う。
	function removeRemotePlayer(id: string) {
		const remote = remotePlayers.get(id);
		if (!remote) {
			return;
		}

		removeRemote(remote);
		remotePlayers.delete(id);
	}

	// 今は毎フレーム動かすシーン側の処理はない。
	// main.ts から同じ形で呼べるように、後でアニメーションを追加できる入口だけ用意している。
	function update() {}

	// 作成した Scene、Camera、Renderer を外へ返す。
	// main.ts が resize や render を行うために、これらの参照が必要。
	return {
		scene,
		camera,
		renderer,
		update,
		upsertRemotePlayer,
		syncRemotePlayers,
		removeRemotePlayer,
	};
}
