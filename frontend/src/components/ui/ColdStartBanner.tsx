import { useEffect, useState } from 'react'
import { onColdStart } from '@/lib/api'
import { Spinner } from './Spinner'

/** Subscribes to the API layer's slow-request signal. */
export function useColdStart(): boolean {
  const [waking, setWaking] = useState(false)
  useEffect(() => onColdStart(setWaking), [])
  return waking
}

export function ColdStartBanner() {
  const waking = useColdStart()
  if (!waking) return null
  return (
    <div
      role="status"
      className="flex items-center justify-center gap-2 bg-amber-100 px-4 py-2 text-xs font-medium text-amber-900 dark:bg-amber-900/40 dark:text-amber-200"
    >
      <Spinner className="size-3.5" />
      Waking up the server… the free tier sleeps between visits, so the first
      request can take a few seconds.
    </div>
  )
}
