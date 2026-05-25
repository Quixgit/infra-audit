import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, Play } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import type { LucideIcon } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { ConnectionCard } from '@/components/ConnectionCard'
import { EmptyState } from '@/components/EmptyState'
import { connectionsApi, bulkApi } from '@/lib/api'

interface Props {
  connType: string | string[]
  title: string
  description: string
  emptyDescription: string
  newConnectionPath: string
  icon: LucideIcon
}

export function ConnectionsPage({
  connType,
  title,
  description,
  emptyDescription,
  newConnectionPath,
  icon,
}: Props) {
  const EmptyIcon = icon
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: allConnections, isLoading } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })

  const connections = (allConnections ?? []).filter((c) =>
    Array.isArray(connType) ? connType.includes(c.conn_type) : c.conn_type === connType
  )

  const bulkMutation = useMutation({
    mutationFn: () => bulkApi.run([...selected]),
    onSuccess: (jobs) => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      qc.invalidateQueries({ queryKey: ['dashboard'] })
      toast.success(`Started ${jobs.length} audit${jobs.length !== 1 ? 's' : ''}`)
      setSelected(new Set())
    },
    onError: () => toast.error('Failed to start audits'),
  })

  const toggleSelect = (id: string) =>
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })

  const allSelected = connections.length > 0 && connections.every((c) => selected.has(c.id))

  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set())
    } else {
      setSelected(new Set(connections.map((c) => c.id)))
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">{title}</h1>
          <p className="text-sm text-muted-foreground mt-1">{description}</p>
        </div>
        <div className="flex items-center gap-2">
          {selected.size > 0 && (
            <Button
              variant="outline"
              size="sm"
              onClick={() => bulkMutation.mutate()}
              disabled={bulkMutation.isPending}
            >
              <Play className="mr-1.5 h-3.5 w-3.5" />
              {bulkMutation.isPending ? 'Starting…' : `Run ${selected.size} selected`}
            </Button>
          )}
          <Button size="sm" onClick={() => navigate(newConnectionPath)}>
            <Plus className="mr-1.5 h-3.5 w-3.5" />
            New Connection
          </Button>
        </div>
      </div>

      {isLoading ? (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} className="h-48 rounded-lg bg-muted animate-pulse" />
          ))}
        </div>
      ) : connections.length === 0 ? (
        <EmptyState
          icon={EmptyIcon}
          title="No connections yet"
          description={emptyDescription}
          action={{ label: 'Add connection', onClick: () => navigate(newConnectionPath) }}
        />
      ) : (
        <div className="space-y-4">
          {connections.length > 1 && (
            <div className="flex items-center gap-2">
              <input
                type="checkbox"
                checked={allSelected}
                onChange={toggleAll}
                className="h-4 w-4 rounded border-border accent-indigo-500"
                id="select-all"
              />
              <label
                htmlFor="select-all"
                className="text-sm text-muted-foreground cursor-pointer"
              >
                Select all
                {selected.size > 0 && (
                  <span className="ml-1.5 text-muted-foreground/60">— {selected.size} selected</span>
                )}
              </label>
            </div>
          )}
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
            {connections.map((conn) => (
              <ConnectionCard
                key={conn.id}
                connection={conn}
                selected={selected.has(conn.id)}
                onToggleSelect={() => toggleSelect(conn.id)}
              />
            ))}
          </div>
        </div>
      )}
    </div>
  )
}
