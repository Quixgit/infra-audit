import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import {
  FileText, ClipboardList, BookText, BarChart3, ListChecks, Loader2,
} from 'lucide-react'
import { useQuery } from '@tanstack/react-query'
import { toast } from 'sonner'
import { Button } from '@/components/ui/button'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from '@/components/ui/select'
import { DownloadButtons } from '@/components/DownloadButtons'
import { FindingsBadges } from '@/components/FindingsBadges'
import { EmptyState } from '@/components/EmptyState'
import { jobsApi, connectionsApi, aggregatedFindingsApi, complianceApi } from '@/lib/api'
import { formatDate } from '@/lib/utils'
import {
  generateFrameworkReport,
  generateExecutiveSummary,
  generateRemediationPlan,
  openReport,
} from '@/lib/reportGenerators'

const reportTypes = [
  {
    id: 'soc2',
    name: 'SOC 2 Readiness Report',
    description: 'Control coverage and gap analysis mapped to SOC 2 Type II trust service criteria.',
    icon: ClipboardList,
    iconBg: 'bg-blue-100 dark:bg-blue-950/40',
    iconColor: 'text-blue-500',
  },
  {
    id: 'iso27001',
    name: 'ISO 27001 Gap Report',
    description: 'Annex A control mapping showing which controls are met, partial, or not yet met.',
    icon: BookText,
    iconBg: 'bg-purple-100 dark:bg-purple-950/40',
    iconColor: 'text-purple-500',
  },
  {
    id: 'exec',
    name: 'Executive Summary',
    description: 'One-page security posture overview with compliance scores and top open findings.',
    icon: BarChart3,
    iconBg: 'bg-emerald-100 dark:bg-emerald-950/40',
    iconColor: 'text-emerald-500',
  },
  {
    id: 'remediation',
    name: 'Remediation Plan',
    description: 'Prioritized action plan for all open findings with recommended timelines.',
    icon: ListChecks,
    iconBg: 'bg-orange-100 dark:bg-orange-950/40',
    iconColor: 'text-orange-500',
  },
]

export function Reports() {
  const navigate = useNavigate()
  const [connFilter, setConnFilter] = useState('all')
  const [generatingId, setGeneratingId] = useState<string | null>(null)

  const { data: jobs = [], isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: jobsApi.list,
  })

  const { data: connections = [] } = useQuery({
    queryKey: ['connections'],
    queryFn: connectionsApi.list,
  })

  const doneJobs = jobs
    .filter((j) => j.status === 'done')
    .filter((j) => connFilter === 'all' || j.connection_id === connFilter)

  const grouped: Record<string, { name: string; connId: string; jobs: typeof doneJobs }> = {}
  for (const job of doneJobs) {
    if (!grouped[job.connection_id]) {
      grouped[job.connection_id] = { name: job.connection_name, connId: job.connection_id, jobs: [] }
    }
    grouped[job.connection_id].jobs.push(job)
  }

  const connName =
    connFilter === 'all'
      ? 'All connections'
      : (connections.find((c) => c.id === connFilter)?.name ?? connFilter)

  const reportDate = new Date().toLocaleDateString('en-US', {
    year: 'numeric',
    month: 'long',
    day: 'numeric',
  })

  async function handleGenerate(id: string) {
    setGeneratingId(id)
    try {
      if (id === 'soc2' || id === 'iso27001') {
        const fw = await complianceApi.getFramework(id)
        openReport(generateFrameworkReport(fw, connFilter, connName, reportDate))
      } else if (id === 'exec') {
        const [findings, frameworks] = await Promise.all([
          aggregatedFindingsApi.list(),
          complianceApi.listFrameworks(),
        ])
        openReport(generateExecutiveSummary(findings, frameworks, jobs, connFilter, connName, reportDate))
      } else if (id === 'remediation') {
        const findings = await aggregatedFindingsApi.list()
        openReport(generateRemediationPlan(findings, connFilter, connName, reportDate))
      }
    } catch {
      toast.error('Failed to generate report')
    } finally {
      setGeneratingId(null)
    }
  }

  return (
    <div className="space-y-6">
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold">Reports</h1>
          <p className="text-sm text-muted-foreground mt-1">
            Download audit reports or generate compliance and executive reports
          </p>
        </div>
        {connections.length > 0 && (
          <Select value={connFilter} onValueChange={setConnFilter}>
            <SelectTrigger className="h-8 w-52 text-xs">
              <SelectValue placeholder="All connections" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="all">All connections</SelectItem>
              {connections.map((c) => (
                <SelectItem key={c.id} value={c.id}>
                  {c.name}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        )}
      </div>

      {/* Generated report types */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Generate Reports</span>
          <div className="flex-1 h-px bg-border/50" />
        </div>
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
          {reportTypes.map((r) => (
            <div
              key={r.id}
              className="flex items-center gap-4 rounded-lg border bg-card p-4 transition-all hover:shadow-md hover:border-border/80"
            >
              <div
                className={`flex h-10 w-10 shrink-0 items-center justify-center rounded-xl ${r.iconBg} ${r.iconColor}`}
              >
                <r.icon className="h-5 w-5" />
              </div>
              <div className="flex-1 min-w-0">
                <p className="text-sm font-semibold leading-tight">{r.name}</p>
                <p className="text-xs text-muted-foreground mt-0.5 leading-snug">{r.description}</p>
              </div>
              <Button
                size="sm"
                variant="outline"
                className="shrink-0 text-xs"
                disabled={generatingId === r.id}
                onClick={() => handleGenerate(r.id)}
              >
                {generatingId === r.id ? (
                  <>
                    <Loader2 className="h-3 w-3 mr-1.5 animate-spin" />
                    Generating…
                  </>
                ) : (
                  'Generate HTML'
                )}
              </Button>
            </div>
          ))}
        </div>
      </div>

      {/* Grouped audit reports */}
      <div>
        <div className="flex items-center gap-2 mb-3">
          <span className="text-[10px] font-semibold uppercase tracking-widest text-muted-foreground/50">Audit Reports</span>
          <div className="flex-1 h-px bg-border/50" />
        </div>
        {isLoading ? (
          <div className="space-y-4">
            {[...Array(2)].map((_, i) => (
              <div key={i} className="h-32 rounded-lg bg-muted animate-pulse" />
            ))}
          </div>
        ) : Object.keys(grouped).length === 0 ? (
          <EmptyState
            icon={FileText}
            title="No completed audits"
            description="Run an audit to generate downloadable reports."
          />
        ) : (
          Object.values(grouped).map(({ name, connId, jobs: connJobs }) => (
            <Card key={connId} className="mb-4">
              <CardHeader className="pb-2">
                <CardTitle className="text-base">{name}</CardTitle>
              </CardHeader>
              <CardContent className="p-0">
                <div className="divide-y divide-border">
                  {connJobs.map((job) => (
                    <div key={job.id} className="flex items-center gap-4 px-6 py-3">
                      <button
                        className="text-sm font-medium hover:underline text-left shrink-0 w-40"
                        onClick={() => navigate(`/jobs/${job.id}`)}
                      >
                        {formatDate(job.started_at)}
                      </button>
                      <div className="flex-1">
                        <FindingsBadges
                          critical={job.findings_critical}
                          high={job.findings_high}
                          medium={job.findings_medium}
                          low={job.findings_low}
                        />
                      </div>
                      <DownloadButtons jobId={job.id} />
                    </div>
                  ))}
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>
    </div>
  )
}
