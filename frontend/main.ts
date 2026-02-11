import { app, BrowserWindow, Menu } from 'electron'
// @ts-ignore
import path from 'node:path'
import { fileURLToPath } from 'node:url'

// @ts-ignore
const __dirname = path.dirname(fileURLToPath(import.meta.url))

const isDev: boolean = process.env.NODE_ENV === 'development' || !app.isPackaged

const createWindow = (): void => {
    const win = new BrowserWindow({
        width: 800,
        height: 600,
        webPreferences: {
            preload: path.join(__dirname, 'preload.js'),
        }
    })

    const menu = Menu.buildFromTemplate([
        { role: 'reload' },
        { role: 'forceReload'},
        { role: 'toggleDevTools'}
    ])
    win.webContents.on('context-menu', (_event, params) => {
        menu.popup()
    })

    if (isDev) {
        win.loadURL('http://localhost:5173').catch(err => console.error('Load URL error:', err))
    } else {
        win.loadFile(path.join(__dirname, 'dist/index.html')).catch(err => console.error('Load file error:', err))
    }
}

app.whenReady().then(() => {
    createWindow()

    app.on('activate', () => {
        if (BrowserWindow.getAllWindows().length === 0) {
            createWindow()
        }
    })
})

app.on('window-all-closed', () => {
    if (process.platform !== 'darwin') {
        app.quit()
    }
})
