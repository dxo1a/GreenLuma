window.addEventListener('DOMContentLoaded', () => {
    const replaceText = (selector: string, text: string | undefined): void => {
        const element = document.getElementById(selector)
        if (element) element.innerText = text ?? ''
    }

    for (const type of ['chrome', 'node', 'electron'] as const) {
        replaceText(`${type}-version`, process.versions[type])
    }
})
