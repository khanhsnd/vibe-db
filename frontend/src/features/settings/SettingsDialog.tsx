import { useState, type ReactNode } from 'react'
import { SlidersHorizontal } from 'lucide-react'

import { PAGE_SIZES, QUERY_ROW_LIMITS } from '@/bridge/rows'
import { cn } from '@/lib/cn'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Checkbox, Field, SelectInput } from '@/shared/ui/Field'
import { useSettingsStore, type Settings } from '@/stores/settings'

import { FontPicker } from './FontPicker'

const SECTIONS = ['General', 'Editor', 'Records'] as const

type Section = (typeof SECTIONS)[number]

interface SectionProps {
  settings: Settings
  update: (part: Partial<Settings>) => void
}

interface SettingsDialogProps {
  open: boolean
  onOpenChange: (open: boolean) => void
}

/**
 * Every preference the reader has, behind the gear in the command bar: a rail of sections on the
 * left and the one in force on the right, which is the shape a reader already knows an options
 * dialog by. A pick is **applied as it is made** rather than behind an OK, because the app behind
 * the dialog is the preview — so the footer offers Restore Defaults and a way out, and nothing
 * that could leave a half-set state.
 */
export function SettingsDialog({ open, onOpenChange }: SettingsDialogProps) {
  const [section, setSection] = useState<Section>('General')
  const settings = useSettingsStore((s) => s.settings)
  const update = useSettingsStore((s) => s.update)
  const restoreDefaults = useSettingsStore((s) => s.restoreDefaults)

  return (
    <Dialog
      open={open}
      onOpenChange={onOpenChange}
      title="Options"
      description="Applied as they are set, and kept for the next run."
      icon={<SlidersHorizontal size={15} />}
      width={720}
      bodyClassName="p-0"
      footer={
        <>
          <Button variant="subtle" onClick={restoreDefaults}>
            Restore Defaults
          </Button>
          <Button variant="primary" onClick={() => onOpenChange(false)}>
            Done
          </Button>
        </>
      }
    >
      <div className="flex min-h-[380px]">
        <nav className="w-40 shrink-0 border-r border-line bg-panel py-2">
          {SECTIONS.map((name) => (
            <button
              key={name}
              type="button"
              onClick={() => setSection(name)}
              className={cn(
                'block w-full px-3 py-1.5 text-left text-sm',
                name === section
                  ? 'bg-selected text-fg'
                  : 'text-muted hover:bg-hover hover:text-fg',
              )}
            >
              {name}
            </button>
          ))}
        </nav>

        <div className="min-w-0 flex-1 px-4 py-3.5">
          {section === 'General' ? <General settings={settings} update={update} /> : null}
          {section === 'Editor' ? <Editor settings={settings} update={update} /> : null}
          {section === 'Records' ? <Records settings={settings} update={update} /> : null}
        </div>
      </div>
    </Dialog>
  )
}

function Group({ title, children }: { title: string; children: ReactNode }) {
  return (
    <section className="flex flex-col gap-3">
      <h3 className="text-xs font-medium tracking-wide text-muted uppercase">{title}</h3>
      {children}
    </section>
  )
}

function General({ settings, update }: SectionProps) {
  return (
    <Group title="Appearance">
      <Field label="Theme" inline>
        {(controlId) => (
          <div className="w-32">
            <SelectInput
              id={controlId}
              value={settings.theme}
              onChange={(event) => update({ theme: event.target.value === 'light' ? 'light' : 'dark' })}
            >
              <option value="dark">Dark</option>
              <option value="light">Light</option>
            </SelectInput>
          </div>
        )}
      </Field>
    </Group>
  )
}

function Editor({ settings, update }: SectionProps) {
  return (
    <div className="flex flex-col gap-5">
      <Group title="Data font">
        <FontPicker
          font={settings.font}
          onChange={(part) => update({ font: { ...settings.font, ...part } })}
        />
      </Group>

      <Group title="SQL editor">
        <Checkbox
          label="Use word wrap"
          checked={settings.wordWrap}
          onChange={(event) => update({ wordWrap: event.target.checked })}
        />
      </Group>
    </div>
  )
}

function Records({ settings, update }: SectionProps) {
  return (
    <Group title="What a new tab starts with">
      <Field label="Records per page" inline hint="A table read and a query result alike.">
        {(controlId) => (
          <div className="w-32">
            <SelectInput
              id={controlId}
              value={String(settings.pageSize)}
              onChange={(event) => update({ pageSize: Number(event.target.value) })}
            >
              {PAGE_SIZES.map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </SelectInput>
          </div>
        )}
      </Field>

      <Field label="Query row limit" inline hint="What a query tab caps its read at.">
        {(controlId) => (
          <div className="w-32">
            <SelectInput
              id={controlId}
              value={String(settings.queryRows)}
              onChange={(event) => update({ queryRows: Number(event.target.value) })}
            >
              {QUERY_ROW_LIMITS.map((rows) => (
                <option key={rows} value={rows}>
                  {rows}
                </option>
              ))}
            </SelectInput>
          </div>
        )}
      </Field>
    </Group>
  )
}
