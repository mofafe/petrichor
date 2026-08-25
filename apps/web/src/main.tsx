import { createRoot } from "react-dom/client";
import { App } from "./App";
import { ROOM_PATH } from "./app/constants";
import "./styles/base.css";

const app = document.querySelector<HTMLDivElement>("#app");

if (!app) {
	throw new Error("Missing app root");
}

document.body.dataset.view =
	window.location.pathname === ROOM_PATH ? "room" : "entry";

createRoot(app).render(<App />);
