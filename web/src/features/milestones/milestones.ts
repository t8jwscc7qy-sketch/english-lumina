import { useQuery } from '@tanstack/react-query'
import { getMilestones } from '@/api/client'

export type { Milestone, MilestoneList } from '@/api/client'
import type { Milestone } from '@/api/client'

export const milestonesKey = ['milestones'] as const

/** Вехи. Открываются после занятий (итог дня, «Прогресс»), поэтому данные всегда перечитываются. */
export function useMilestones() {
  return useQuery({
    queryKey: milestonesKey,
    queryFn: getMilestones,
    retry: false,
    staleTime: 0,
    refetchOnWindowFocus: false,
  })
}

/** Ближайшая недостигнутая веха по линейкам серии, часов и ответов; недостигнутые «первое» — все (они независимы). */
export function nextPerKind(milestones: Milestone[]): Milestone[] {
  const seen = new Set<string>()
  const out: Milestone[] = []
  for (const m of milestones) {
    if (m.reachedOn) continue
    if (m.kind === 'first') {
      out.push(m)
      continue
    }
    if (seen.has(m.kind)) continue
    seen.add(m.kind)
    out.push(m)
  }
  return out
}

/** Новые вехи в порядке сервера (самая крупная первой); неизвестные id пропускаются. */
export function sortNew(milestones: Milestone[], ids: string[]): Milestone[] {
  const byId = new Map(milestones.map((m) => [m.id, m]))
  return ids.flatMap((id) => {
    const m = byId.get(id)
    return m ? [m] : []
  })
}
