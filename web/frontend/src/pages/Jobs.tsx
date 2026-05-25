import { useState } from 'react'
import { Briefcase, CheckCircle2, AlertTriangle, Play, XCircle } from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import {
  Table,
  TableBody,
  TableHead,
  TableHeader,
  TableRow,
} from '@/components/ui/table'
import { Badge } from '@/components/ui/badge'
import { Card, CardContent } from '@/components/ui/card'
import { AuditJobRow } from '@/components/AuditJobRow'
import { EmptyState } from '@/components/EmptyState'
import { jobsApi, type AuditJob } from '@/lib/api'

type Filter = 'all' | 'running' | 'done' | 'failed'

function StatCard({
  label,
  value,
  icon: Icon,
  iconBg,
  iconColor,
  valueColor,
}: {
  label: string
  value: number
  icon: React.ElementType
  iconBg: string
  iconColor: string
  valueColor?: string
}) {
  return (
    <Card className="relative overflow-hidden">
      <CardContent className="pt-5 pb-4">
        <div className="flex items-start justify-between gap-3">
          <div className="flex-1 min-w-0">
            <p className="text-xs text-muted-foreground font-medium mb-1.5 uppercase tracking-wide">{label}</p>
            <p className={`text-3xl font-bold leading-none ${valueColor ?? ''}`}>{value}</p>
          </div>
          <div className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${iconBg}`}>
            <Icon className={`h-5 w-5 ${iconColor}`} />
          </div>
        </div>
      </CardContent>
    </Card>
  )
}

export function Jobs() {
  const [filter, setFilter] = useState<Filter>('all')

  const { data: jobs, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: jobsApi.list,
    refetchInterval: (query) => {
      const data = query.state.data as AuditJob[] | undefined
      if (data?.some((j) => j.status === 'running' || j.status === 'pending')) return 5000
      return false
    },
  })

  const filters: { value: Filter; label: string }[] = [
    { value: 'all',     label: 'All' },
    { value: 'running', label: 'Running' },
    { value: 'done',    label: 'Done' },
    { value: 'failed',  label: 'Failed' },
  ]

  const filtered =
    filter === 'all' ? (jobs ?? []) : (jobs ?? []).filter((j) => j.status === filter)

  const runningCount = (jobs ?? []).filter((j) => j.status === 'running' || j.status === 'pending').length
  const doneCount    = (jobs ?? []).filter((j) => j.status === 'done').length
  const failedCount  = (jobs ?? []).filter((j) => j.status === 'failed').length

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-2xl font-bold">Audit Jobs</h1>
        <p className="text-sm text-muted-foreground mt-1">
          View and manage infrastructure audit runs
        </p>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-2 gap-4 sm:grid-cols-4">
        <StatCard
          label="Total"
          value={jobs?.length ?? 0}
          icon={Briefcase}
          iconBg="bg-indigo-500/10"
          iconColor="text-indigo-400"
        />
        <StatCard
          label="Running"
          value={runningCount}
          icon={Play}
          iconBg="bg-blue-500/10"
          iconColor="text-blue-400"
          valueColor={runningCount > 0 ? 'text-blue-400' : undefined}
        />
        <StatCard
          label="Completed"
          value={doneCount}
          icon={CheckCircle2}
          iconBg="bg-green-500/10"
          iconColor="text-green-400"
          valueColor={doneCount > 0 ? 'text-green-400' : undefined}
        />
        <StatCard
          label="Failed"
          value={failedCount}
          icon={XCircle}
          iconBg="bg-red-500/10"
          iconColor="text-red-400"
          valueColor={failedCount > 0 ? 'text-red-400' : undefined}
        />
      </div>

      {/* Filter chips */}
      <div className="flex gap-2">
        {filters.map((f) => {
          const count =
            f.value === 'all'
              ? (jobs?.length ?? 0)
              : (jobs?.filter((j) => j.status === f.value).length ?? 0)
          return (
            <button
              key={f.value}
              onClick={() => setFilter(f.value)}
              className={`flex items-center gap-1.5 rounded-full px-3 py-1 text-sm font-medium transition-colors ${
                filter === f.value
                  ? 'bg-indigo-500/20 text-indigo-400'
                  : 'bg-muted text-muted-foreground hover:bg-muted/80'
              }`}
            >
              {f.label}
              <Badge variant="secondary" className="h-5 min-w-5 px-1 text-xs">
                {count}
              </Badge>
            </button>
          )
        })}
      </div>

      {isLoading ? (
        <div className="space-y-2">
          {[...Array(4)].map((_, i) => (
            <div key={i} className="h-14 rounded-md bg-muted animate-pulse" />
          ))}
        </div>
      ) : !filtered.length ? (
        <EmptyState
          icon={Briefcase}
          title="No jobs found"
          description={
            filter === 'all'
              ? 'Run an audit from a connection to see results here.'
              : `No ${filter} jobs.`
          }
        />
      ) : (
        <div className="rounded-lg border overflow-hidden">
          <Table>
            <TableHeader>
              <TableRow className="bg-muted/40">
                <TableHead className="text-xs font-medium text-muted-foreground">Connection</TableHead>
                <TableHead className="text-xs font-medium text-muted-foreground">Started</TableHead>
                <TableHead className="text-xs font-medium text-muted-foreground">Duration</TableHead>
                <TableHead className="text-xs font-medium text-muted-foreground">Status</TableHead>
                <TableHead className="text-xs font-medium text-muted-foreground">Findings</TableHead>
                <TableHead className="text-xs font-medium text-muted-foreground text-right">Actions</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {filtered.map((job) => (
                <AuditJobRow key={job.id} job={job} />
              ))}
            </TableBody>
          </Table>
        </div>
      )}
    </div>
  )
}
