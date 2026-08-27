import React from "react";
import { createRoot } from "react-dom/client";
import { DebugApp } from "./DebugApp";
// import "./debug.css";

// debug.html にある React 用の mount 先を探す。
// 通常の 3D ルームとは別ページなので、ここだけ React で起動している。
const app = document.querySelector<HTMLDivElement>("#debug-app");

if (!app) {
	throw new Error("Missing debug app");
}

// StrictMode は開発中に副作用のミスを見つけやすくする React のモード。
createRoot(app).render(
	<React.StrictMode>
		<DebugApp />
	</React.StrictMode>,
);
