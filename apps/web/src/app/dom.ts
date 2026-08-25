// 必ず存在してほしい DOM 要素を取得する小さなヘルパー。
// `document.querySelector()` は見つからないと null を返すため、
// ここで早めにエラーにしておくと HTML と TypeScript のズレに気づきやすい。
export function queryRequired<TElement extends Element>(
	selector: string,
): TElement {
	const element = document.querySelector<TElement>(selector);
	if (!element) {
		throw new Error(`Missing element: ${selector}`);
	}
	return element;
}

// 入力欄で Enter を押した時に同じ処理を実行するためのヘルパー。
// entry.ts では「ボタンを押す」と「Enter を押す」を同じ joinRoom() に揃えている。
export function onEnter(element: HTMLElement, handler: () => void) {
	element.addEventListener("keydown", (event) => {
		if (event.key === "Enter") {
			handler();
		}
	});
}
