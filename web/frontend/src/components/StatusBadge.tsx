import { cn } from '@/lib/utils'
import { Loader2, CheckCircle2, XCircle, Clock } from 'lucide-react'

type Status = 'pending' | 'running' | 'done' | 'failed'

interface Props {
  status: Status
  className?: string
}

const config: Record<Status, { label: string; className: string; Icon: React.ElementType }> = {
  pending: { label: 'Pending', className: 'text-muted-foreground bg-muted', Icon: Clock },
  running: { label: 'Running', className: 'text-blue-400 bg-blue-500/10', Icon: Loader2 },
  done: { label: 'Done', className: 'text-green-400 bg-green-500/10', Icon: CheckCircle2 },
  failed: { label: 'Failed', className: 'text-red-400 bg-red-500/10', Icon: XCircle },
}

export function StatusBadge({ status, className }: Props) {
  const { label, className: cls, Icon } = config[status] ?? config.pending
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 rounded-full px-2.5 py-0.5 text-xs font-medium',
        cls,
        className
      )}
    >
      <Icon className={cn('h-3 w-3', status === 'running' && 'animate-spin')} />
      {label}
    </span>
  )
}
