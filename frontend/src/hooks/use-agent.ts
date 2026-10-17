// SPDX-License-Identifier: AGPL-3.0-or-later
import { useCallback, useEffect, useState } from 'react'
import { Api, onDataChanged } from '../api'

/** Ids of the profiles an AI agent is driving now, over the MCP endpoint. */
export function useAgentProfiles(): Set<string> {
  const [ids, setIds] = useState<Set<string>>(new Set())
  const reload = useCallback(() => {
    Promise.resolve().then(() => Api.AgentProfiles()).then((a) => setIds(new Set(a ?? []))).catch(() => {})
  }, [])
  useEffect(reload, [reload])
  useEffect(() => onDataChanged(['agent'], reload), [reload])
  return ids
}
