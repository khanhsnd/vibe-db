import { useState } from 'react'
import { Bookmark } from 'lucide-react'

import { ENGINES } from '@/bridge/connections'
import type { SnippetInput } from '@/bridge/snippets'
import { useSchemaSubmit } from '@/features/schema/useSchemaSubmit'
import { Banner } from '@/shared/ui/Banner'
import { Button } from '@/shared/ui/Button'
import { Dialog } from '@/shared/ui/Dialog'
import { Field, SelectInput, TextArea, TextInput } from '@/shared/ui/Field'
import { ICON } from '@/shared/ui/menuIcons'
import { useSnippetsStore } from '@/stores/snippets'

interface SnippetDialogProps {
  /** What the boxes open on. A snippet being edited carries its own name as `previous`. */
  draft: SnippetInput
  onClose: () => void
}

/**
 * One snippet, filled in. It is a dialog rather than a tab because it is four boxes and nothing is
 * read from a server to draw it — the same reason a rename is one; see `stores/schema.ts`.
 *
 * **New and Edit are one dialog**, told apart by whether the draft carries a `previous` name. They
 * ask the same four questions, and a save under a name the library already holds replaces that entry
 * either way — so a second component would be the same form with a different verb on the button.
 */
export function SnippetDialog({ draft, onClose }: SnippetDialogProps) {
  const [form, setForm] = useState(draft)
  const save = useSnippetsStore((s) => s.save)

  // The same hook every schema dialog uses: run it, close on success, and keep a rejection on screen
  // so the reader corrects it rather than losing what they typed.
  const { submit, running, error, fieldError } = useSchemaSubmit(onClose)

  const editing = draft.previous !== ''
  const set = (part: Partial<SnippetInput>) => setForm((held) => ({ ...held, ...part }))

  return (
    <Dialog
      open
      onOpenChange={onClose}
      title={editing ? 'Edit Snippet' : 'New Snippet'}
      description="Kept on this machine, and offered by name wherever you write a statement."
      icon={<Bookmark size={15} />}
      width={560}
      footer={
        <>
          <Button onClick={onClose}>Cancel</Button>
          <Button
            variant="primary"
            icon={ICON.saveAs}
            disabled={running}
            onClick={() => submit(() => save(form))}
          >
            {running ? 'Saving…' : 'Save Snippet'}
          </Button>
        </>
      }
    >
      <div className="flex flex-col gap-3">
        <Field
          label="Name"
          hint="What you type to reach it in the editor."
          error={fieldError('snippet')}
        >
          {(id, invalid) => (
            <TextInput
              id={id}
              invalid={invalid}
              value={form.name}
              autoFocus
              placeholder="recent orders"
              onChange={(e) => set({ name: e.target.value })}
            />
          )}
        </Field>

        <Field label="Description" hint="Optional — what the name does not say.">
          {(id) => (
            <TextInput
              id={id}
              value={form.description}
              placeholder="Everything placed in the last week, newest first"
              onChange={(e) => set({ description: e.target.value })}
            />
          )}
        </Field>

        <Field
          label="Engine"
          hint="Which editors offer it. A snippet for every engine is offered in all of them."
        >
          {(id) => (
            <SelectInput
              id={id}
              value={form.engine}
              onChange={(e) => set({ engine: e.target.value })}
            >
              <option value="">Every engine</option>
              {ENGINES.map((info) => (
                <option key={info.value} value={info.value}>
                  {info.label}
                </option>
              ))}
            </SelectInput>
          )}
        </Field>

        {/* The body is the reader's own text and is kept byte for byte: nothing here lays it out,
            splits it or checks it, because what a statement *is* is the engine's answer and this is
            not in front of one. It reads in the data font, so what is typed lines up with what the
            editor will show. */}
        <Field
          label="Statement"
          hint="Wrap the parts that change in ${…} and Tab walks between them when it is inserted."
          error={error?.code === 'EMPTY_SNIPPET' ? error.message : undefined}
        >
          {(id, invalid) => (
            <TextArea
              id={id}
              invalid={invalid}
              value={form.body}
              rows={10}
              placeholder={'SELECT *\nFROM orders\nWHERE placed > ${date}'}
              onChange={(e) => set({ body: e.target.value })}
              className="data-font scroll-thin resize-y font-mono text-xs"
            />
          )}
        </Field>

        {error && error.code !== 'EMPTY_SNIPPET' && error.code !== 'INVALID_NAME' ? (
          <Banner tone="danger" text={error.message} />
        ) : null}
      </div>
    </Dialog>
  )
}
