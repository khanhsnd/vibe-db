import { ClipboardGetText, ClipboardSetText } from '@wailsjs/runtime/runtime'

/**
 * The host's clipboard, reached through Wails rather than `navigator.clipboard`: reading in a
 * WebView2 needs a `clipboard-read` permission the page has no way to ask for. The browser API
 * is the fallback, for a page served outside the shell.
 */
export async function readText(): Promise<string> {
  try {
    return await ClipboardGetText()
  } catch {
    return await browserRead()
  }
}

export async function copyText(text: string): Promise<void> {
  try {
    await ClipboardSetText(text)
  } catch {
    await browserWrite(text)
  }
}

async function browserRead(): Promise<string> {
  try {
    return await navigator.clipboard.readText()
  } catch {
    return ''
  }
}

async function browserWrite(text: string): Promise<void> {
  try {
    await navigator.clipboard.writeText(text)
  } catch {
    copyViaTextarea(text)
  }
}

function copyViaTextarea(text: string) {
  const area = document.createElement('textarea')
  area.value = text
  area.readOnly = true
  area.style.position = 'fixed'
  area.style.top = '-100vh'
  area.style.opacity = '0'

  document.body.append(area)
  area.select()
  document.execCommand('copy')
  area.remove()
}
