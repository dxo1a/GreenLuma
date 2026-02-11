import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'
import electron from 'vite-plugin-electron'
import renderer from 'vite-plugin-electron-renderer'

// https://vite.dev/config/
export default defineConfig({
    base: './',
    plugins: [
        vue(),
        vueDevTools(),
        renderer(),
        electron([
            {
                entry: 'main.ts',
                vite: {
                    build: {
                        target: 'node24',
                        outDir: 'dist-electron'
                    }
                }
            },
            {
                entry: 'preload.ts',
                vite: {
                    build: {
                        target: 'node24',
                        outDir: 'dist-electron'
                    }
                }
            }
        ])
    ],
    resolve: {
        alias: {
            '@': fileURLToPath(new URL('./src', import.meta.url))
        },
    },
})
