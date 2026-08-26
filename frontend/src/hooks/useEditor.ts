import { useCallback, useState } from 'react'

export interface EditorState<T> {
  isOpen: boolean
  /** `null` while creating, the record while editing. */
  editing: T | null
  openCreate: () => void
  openEdit: (item: T) => void
  close: () => void
}

/** Shared open/close + "which record" state for add/edit modals. */
export function useEditor<T>(): EditorState<T> {
  const [isOpen, setIsOpen] = useState(false)
  const [editing, setEditing] = useState<T | null>(null)

  const openCreate = useCallback(() => {
    setEditing(null)
    setIsOpen(true)
  }, [])

  const openEdit = useCallback((item: T) => {
    setEditing(item)
    setIsOpen(true)
  }, [])

  const close = useCallback(() => {
    setIsOpen(false)
    setEditing(null)
  }, [])

  return { isOpen, editing, openCreate, openEdit, close }
}

export interface ConfirmState<T> {
  target: T | null
  request: (item: T) => void
  cancel: () => void
}

/** Shared "pending delete" state for confirmation dialogs. */
export function useConfirm<T>(): ConfirmState<T> {
  const [target, setTarget] = useState<T | null>(null)
  return {
    target,
    request: useCallback((item: T) => setTarget(item), []),
    cancel: useCallback(() => setTarget(null), []),
  }
}
