import { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Trash2, Eye } from 'lucide-react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { toast } from 'sonner'
import { TableRow, TableCell } from '@/components/ui/table'
import { Button } from '@/components/ui/button'
import { StatusBadge } from './StatusBadge'
import { FindingsBadges } from './FindingsBadges'
import { ConfirmDialog } from './ConfirmDialog'
import { jobsApi, type AuditJob } from '@/lib/api'
import { formatDate, formatDuration } from '@/lib/utils'

interface Props {
  job: AuditJob
}

export function AuditJobRow({ job }: Props) {
  const navigate = useNavigate()
  const qc = useQueryClient()
  const [deleteOpen, setDeleteOpen] = useState(false)

  const deleteMutation = useMutation({
    mutationFn: () => jobsApi.delete(job.id),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['jobs'] })
      toast.success('Job deleted')
    },
    onError: () => toast.error('Failed to delete job'),
  })

  return (
    <>
      <TableRow className="cursor-pointer" onClick={() => navigate(`/jobs/${job.id}`)}>
        <TableCell className="font-medium">{job.connection_name}</TableCell>
        <TableCell className="text-sm text-muted-foreground">{formatDate(job.started_at)}</TableCell>
        <TableCell className="text-sm text-muted-foreground">
          {formatDuration(job.started_at, job.finished_at)}
        </TableCell>
        <TableCell>
          <StatusBadge status={job.status} />
        </TableCell>
        <TableCell>
          {job.status === 'done' && (
            <FindingsBadges
              critical={job.findings_critical}
              high={job.findings_high}
              medium={job.findings_medium}
              low={job.findings_low}
            />
          )}
          {job.status === 'failed' && (
            <span className="text-xs text-destructive">{job.error_msg}</span>
          )}
        </TableCell>
        <TableCell
          className="text-right"
          onClick={(e) => e.stopPropagation()}
        >
          <div className="flex justify-end gap-1">
            <Button size="icon" variant="ghost" onClick={() => navigate(`/jobs/${job.id}`)}>
              <Eye className="h-4 w-4" />
            </Button>
            <Button
              size="icon"
              variant="ghost"
              className="text-destructive hover:text-destructive"
              onClick={() => setDeleteOpen(true)}
            >
              <Trash2 className="h-4 w-4" />
            </Button>
          </div>
        </TableCell>
      </TableRow>

      <ConfirmDialog
        open={deleteOpen}
        onOpenChange={setDeleteOpen}
        title="Delete job"
        description="Delete this audit job and its report files?"
        onConfirm={() => deleteMutation.mutate()}
        loading={deleteMutation.isPending}
      />
    </>
  )
}
