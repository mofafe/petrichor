type IceServerResponse = {
	iceServers?: RTCIceServer[];
};

// nginx entrypoint が生成する runtime config を優先する。
// ビルド後でも接続先を差し替えられるように、import.meta.env だけに寄せていない。
const RUNTIME_CONFIG =
	window.__PETRICHOR_CONFIG__ ?? window.__FLATTALKING_CONFIG__;
const ICE_API_ORIGIN = RUNTIME_CONFIG?.iceApiOrigin?.trim();
const WORLD_WS_ORIGIN = RUNTIME_CONFIG?.worldWsOrigin?.trim();

// ローカル開発で Go server が立つ想定の origin。
const DEFAULT_LOCAL_API_ORIGIN = "http://localhost:8080";
const LOCAL_HOSTNAMES = new Set(["localhost", "127.0.0.1"]);
const VITE_DEV_PORT_PREFIX = "517";
const IS_VITE_DEV = import.meta.env.DEV;

export async function loadIceServers(
	location: Location = window.location,
): Promise<RTCIceServer[]> {
	// WebRTC が NAT 越えをするための STUN/TURN 設定を server から取得する。
	// マイクを有効化する直前に voice.ts から呼ばれる。
	const response = await fetch(`${iceApiOrigin(location)}/api/iolite/ice`);
	if (!response.ok) {
		throw new Error(`Failed to load ICE servers: ${response.status}`);
	}

	// サーバーが iceServers を返さない場合でも、空配列として扱う。
	// その場合はブラウザ標準の直接接続だけで試す形になる。
	const data = (await response.json()) as IceServerResponse;
	return data.iceServers ?? [];
}

function iceApiOrigin(location: Location): string {
	// 明示設定があるなら最優先。
	// デプロイ環境では API と web の origin が違うことがあるため。
	if (ICE_API_ORIGIN) {
		return ICE_API_ORIGIN;
	}

	// world WebSocket origin だけ設定されている場合は、同じ host を http/https に変換して API origin にする。
	if (WORLD_WS_ORIGIN) {
		return webSocketOriginToHttpOrigin(WORLD_WS_ORIGIN);
	}

	const isViteDevServer = location.port.startsWith(VITE_DEV_PORT_PREFIX);
	const isLocal = LOCAL_HOSTNAMES.has(location.hostname);
	if (IS_VITE_DEV && isViteDevServer && isLocal) {
		// Vite dev 中は vite.config.ts の proxy に /api を流させるため、origin は空文字でよい。
		return "";
	}

	if (isViteDevServer && isLocal) {
		// dev build ではないが 517x で見ている時は、直接 Go server の 8080 を使う。
		return DEFAULT_LOCAL_API_ORIGIN;
	}

	if (isLocal && location.port === "") {
		// localhost を port なしで開いた場合も、API は 8080 とみなす。
		return DEFAULT_LOCAL_API_ORIGIN;
	}

	// 本番で同一 origin 配信なら、そのまま現在の origin の /api/ice を叩く。
	return location.origin;
}

function webSocketOriginToHttpOrigin(origin: string): string {
	// WebSocket の wss/ws は fetch には使えないため、https/http に置き換える。
	const url = new URL(origin);
	url.protocol = url.protocol === "wss:" ? "https:" : "http:";
	return url.origin;
}
