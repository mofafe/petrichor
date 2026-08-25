/// <reference types="vite/client" />

type PetrichorRuntimeConfig = {
	worldWsOrigin?: string;
	iceApiOrigin?: string;
};

interface Window {
	__PETRICHOR_CONFIG__?: PetrichorRuntimeConfig;
	__FLATTALKING_CONFIG__?: PetrichorRuntimeConfig;
}
