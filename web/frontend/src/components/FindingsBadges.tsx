import { cn } from '@/lib/utils'

interface Props {
  critical: number
  high: number
  medium: number
  low: number
  className?: string
}

function Chip({ count, label, color }: { count: number; label: string; color: string }) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1 rounded px-2 py-0.5 text-xs font-semibold',
        color
      )}
      title={`${count} ${label}`}
    >
      <span className="font-bold">{count}</span>
      <span className="opacity-80">{label[0].toUpperCase()}</span>
    </span>
  )
}

export function FindingsBadges({ critical, high, medium, low, className }: Props) {
  return (
    <div className={cn('flex items-center gap-1', className)}>
      <Chip count={critical} label="Critical" color="bg-red-500/20 text-red-400" />
      <Chip count={high} label="High" color="bg-orange-500/20 text-orange-400" />
      <Chip count={medium} label="Medium" color="bg-yellow-500/20 text-yellow-400" />
      <Chip count={low} label="Low" color="bg-blue-500/20 text-blue-400" />
    </div>
  )
}
