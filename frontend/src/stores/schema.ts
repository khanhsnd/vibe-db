import { create } from 'zustand'

/**
 * A schema change waiting on one answer from the reader. Each request carries what it needs to be
 * drawn and a closure that performs the change, so the dialogs know nothing about sessions and the
 * tree menu that raised the request stays the one place a statement is decided.
 *
 * Anything with a *form* in it is not here: a field's shape is edited in the designer tab, which
 * has the room for it. Only a rename, which is one box, and a confirmation, which has to be
 * answered before anything else happens, are modal.
 */
export type SchemaRequest = PromptRequest | ConfirmRequest

export interface PromptRequest {
  kind: 'prompt'
  title: string
  label: string
  value: string
  submitLabel: string
  onSubmit: (value: string) => Promise<void>
}

export interface ConfirmRequest {
  kind: 'confirm'
  title: string
  message: string
  submitLabel: string
  danger: boolean
  onSubmit: () => Promise<void>
}

interface SchemaState {
  request: SchemaRequest | null
  /**
   * The table whose designer should open with a blank field ready — what the tree's Add Field
   * asks for. It is a slot here rather than a field of the tab because the designer may already
   * be open, and a tab that is merely revealed has to hear the second ask too.
   */
  addFieldTo: string | null
  ask: (request: SchemaRequest) => void
  dismiss: () => void
  askAddField: (tableKey: string) => void
  addedField: () => void
}

export const useSchemaStore = create<SchemaState>((set) => ({
  request: null,
  addFieldTo: null,
  ask: (request) => set({ request }),
  dismiss: () => set({ request: null }),
  askAddField: (tableKey) => set({ addFieldTo: tableKey }),
  addedField: () => set({ addFieldTo: null }),
}))
