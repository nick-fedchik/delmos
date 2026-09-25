/// <reference types="vite/client" />

interface ImportMetaEnv {
	readonly VITE_DELMOS_VERSION: string
}

interface ImportMeta {
	readonly env: ImportMetaEnv
}
