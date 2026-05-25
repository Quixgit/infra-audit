import { cn } from '@/lib/utils'

interface Props {
  className?: string
  indeterminate?: boolean
}

export function ProgressBar({ className, indeterminate = true }: Props) {
  if (!indeterminate) return null
  return (
    <div className={cn('relative h-1.5 w-full overflow-hidden rounded-full bg-muted', className)}>
      <div className="absolute inset-0 -translate-x-full animate-[progress_1.5s_ease-in-out_infinite] bg-primary" />
      <style>{`
        @keyframes progress {
          0% { transform: translateX(-100%); }
          100% { transform: translateX(200%); }
        }
      `}</style>
    </div>
  )
}
