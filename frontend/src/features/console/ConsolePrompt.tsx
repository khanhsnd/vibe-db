import { useEffect, useRef, useState, type KeyboardEvent } from 'react'
import { ChevronRight } from 'lucide-react'

/**
 * The box a console is typed at. It behaves the way every console does, which is the whole reason
 * this is not the SQL editor: **Enter sends**, Shift+Enter takes a new line, and the arrows walk
 * back through what has already been sent — the transcript above is that history, so nothing is
 * kept here to make it work.
 *
 * A line recalled and then edited is the reader's own again: walking away from it and back would
 * otherwise throw away the edit, which is the one thing a shell never does.
 *
 * **It grows with what is typed into it** — see `field-sizing` below — because a console is not
 * only for one-liners, and a statement being written over five lines in a box two lines tall is
 * written blind.
 */
export function ConsolePrompt({
  history,
  disabled,
  onSend,
}: {
  history: string[]
  disabled: boolean
  onSend: (sql: string) => void
}) {
  const [text, setText] = useState('')
  /** How far back through the history the reader has walked; -1 is the line they are typing. */
  const [at, setAt] = useState(-1)
  const box = useRef<HTMLTextAreaElement>(null)

  /**
   * The caret starts here and comes back here. Opening a console is asking to type at it, so
   * nothing should have to be clicked first; and the box is *disabled* while a statement is in
   * flight, which takes the focus away — so it is taken back the moment the answer lands, and the
   * reader can type the next line without reaching for the mouse.
   *
   * It asks twice, and the second time is the one that works: a console opened from the tree's
   * menu mounts while that menu is still closing, and a closing Radix menu puts the focus back
   * where it was raised from — over this one, which had already claimed it.
   */
  useEffect(() => {
    if (disabled) return

    const claim = () => box.current?.focus()
    claim()
    const frame = requestAnimationFrame(claim)
    return () => cancelAnimationFrame(frame)
  }, [disabled])

  const recall = (step: number) => {
    const next = Math.min(Math.max(at + step, -1), history.length - 1)
    setAt(next)
    setText(next === -1 ? '' : (history[history.length - 1 - next] ?? ''))
  }

  const send = () => {
    const sql = text.trim()
    if (sql === '' || disabled) return

    onSend(sql)
    setText('')
    setAt(-1)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault()
      send()
      return
    }

    // Only from the ends of the text, so the arrows still move the caret through a line being
    // written over several of them.
    const field = event.currentTarget
    const oneLine = !text.includes('\n')

    if (event.key === 'ArrowUp' && oneLine && field.selectionStart === 0) {
      event.preventDefault()
      recall(1)
    }
    if (event.key === 'ArrowDown' && oneLine && field.selectionEnd === text.length) {
      event.preventDefault()
      recall(-1)
    }
  }

  return (
    <div
      onClick={() => box.current?.focus()}
      className="flex shrink-0 items-start gap-1.5 border-t border-line bg-bar px-2 py-1.5"
    >
      <ChevronRight size={13} className="mt-1 shrink-0 text-accent" />
      {/* `field-sizing-content` is what makes the box follow the statement: it opens four lines
          tall and grows to twelve, past which it scrolls rather than eating the transcript above
          it. The two bounds are the whole of the sizing — no measuring, no resize handler. */}
      <textarea
        ref={box}
        value={text}
        disabled={disabled}
        spellCheck={false}
        placeholder={disabled ? 'Not connected' : 'Statement…'}
        onChange={(e) => setText(e.target.value)}
        onKeyDown={onKeyDown}
        className="scroll-thin field-sizing-content max-h-[12lh] min-h-[4lh] w-full resize-none bg-transparent font-mono text-xs leading-relaxed text-fg outline-none placeholder:text-faint disabled:opacity-50"
      />
    </div>
  )
}
