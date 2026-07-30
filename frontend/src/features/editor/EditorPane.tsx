interface EditorPaneProps {
  value: string
  onChange: (value: string) => void
}

export function EditorPane({ value, onChange }: EditorPaneProps) {
  return (
    <textarea
      value={value}
      onChange={(e) => onChange(e.target.value)}
      spellCheck={false}
      placeholder="SELECT * FROM ..."
      className="scroll-thin block h-full w-full resize-none bg-canvas px-3 py-2 font-mono text-sm leading-relaxed text-fg outline-none placeholder:text-faint"
    />
  )
}
