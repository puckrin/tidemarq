import { useEffect } from 'react'
import { wsClient } from '../api/ws'
import type { WsEvent } from '../api/types'

export function useWsEvents(handler: (e: WsEvent) => void) {
  useEffect(() => {
    const unsub = wsClient.subscribe(handler)
    return () => { unsub() }
  }, [handler])
}
