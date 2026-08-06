const MENUS = ['File', 'Edit', 'View', 'Favorites', 'Tools', 'Window', 'Help']

export function MenuBar() {
  return (
    <nav className="flex h-7 shrink-0 items-center gap-0.5 border-b border-line bg-panel px-1.5">
      {MENUS.map((label) => (
        <button
          key={label}
          type="button"
          className="rounded px-2 py-0.5 text-xs text-fg transition-colors hover:bg-hover"
        >
          {label}
        </button>
      ))}
    </nav>
  )
}
