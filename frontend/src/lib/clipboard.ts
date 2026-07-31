/** Returns '' when the platform refuses the read, which simply means a paste does nothing. */
export async function readText(): Promise<string> {
  try {
    return await navigator.clipboard.readText()
  } catch {
    return ''
  }
}

export async function copyText(text: string): Promise<void> {
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
