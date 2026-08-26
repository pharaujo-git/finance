import { act, renderHook } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { useConfirm, useEditor } from './useEditor'

interface Row {
  id: string
}

describe('useEditor', () => {
  it('starts closed', () => {
    const { result } = renderHook(() => useEditor<Row>())
    expect(result.current.isOpen).toBe(false)
    expect(result.current.editing).toBeNull()
  })

  it('opens for creation with no record', () => {
    const { result } = renderHook(() => useEditor<Row>())
    act(() => result.current.openCreate())
    expect(result.current.isOpen).toBe(true)
    expect(result.current.editing).toBeNull()
  })

  it('opens for editing with the record', () => {
    const { result } = renderHook(() => useEditor<Row>())
    act(() => result.current.openEdit({ id: 'x' }))
    expect(result.current.editing).toEqual({ id: 'x' })
  })

  it('clears the record on close', () => {
    const { result } = renderHook(() => useEditor<Row>())
    act(() => result.current.openEdit({ id: 'x' }))
    act(() => result.current.close())
    expect(result.current.isOpen).toBe(false)
    expect(result.current.editing).toBeNull()
  })
})

describe('useConfirm', () => {
  it('tracks and clears the pending target', () => {
    const { result } = renderHook(() => useConfirm<Row>())
    expect(result.current.target).toBeNull()

    act(() => result.current.request({ id: 'y' }))
    expect(result.current.target).toEqual({ id: 'y' })

    act(() => result.current.cancel())
    expect(result.current.target).toBeNull()
  })
})
