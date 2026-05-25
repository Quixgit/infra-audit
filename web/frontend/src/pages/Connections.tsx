import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Plus, PlugZap, Play, CheckSquare2 } from 'lucide-react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { ConnectionCard } from '@/components/ConnectionCard'
import { EmptyState } from '@/components/EmptyState'
import { connectionsApi, bulkApi } from '@/lib/api'

export function Connections() {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [selected, setSelected] = useState<Set<string>>(new Set())

  const { data: connections, isLoading } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })

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

  const toggleSelect = (id: string) => {
    setSelected((prev) => {
      const next = new Set(prev)
      next.has(id) ? next.delete(id) : next.add(id)
      return next
    })
  }

  const allSelected = connections?.length
    ? connections.every((c) => selected.has(c.id))
    : false

  const toggleAll = () => {
    if (allSelected) {
      setSelected(new Set())
    } else {
      setSelected(new Set(connections?.map((c) => c.id) ?? []))
    }
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Connections</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Manage your cloud and infrastructure connections
          </p>
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
          <Button size="sm" onClick={() => navigate('/connections/new')}>
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
      ) : !connections?.length ? (
        <EmptyState
          icon={PlugZap}
          title="No connections yet"
          description="Add a DigitalOcean, SSL/TLS or code connection to start running infrastructure audits."
          action={{ label: 'Add connection', onClick: () => navigate('/connections/new') }}
        />
      ) : (
        <div className="space-y-4">
          {connections.length > 1 && (
            <div className="flex items-center gap-2 px-0.5">
              <button
                onClick={toggleAll}
                className="flex items-center gap-2 text-sm text-muted-foreground hover:text-foreground transition-colors"
              >
                <CheckSquare2 className={`h-4 w-4 ${allSelected ? 'text-indigo-400' : ''}`} />
                {allSelected ? 'Deselect all' : 'Select all'}
              </button>
              {selected.size > 0 && (
                <span className="text-xs text-muted-foreground/60">
                  — {selected.size} selected
                </span>
              )}
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
