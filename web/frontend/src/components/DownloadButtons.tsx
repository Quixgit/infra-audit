import { FileText, FileCode } from 'lucide-react'
import { Button } from '@/components/ui/button'
import { jobsApi } from '@/lib/api'

interface Props {
  jobId: string
}

export function DownloadButtons({ jobId }: Props) {
  const download = (url: string, filename: string) => {
    const token = localStorage.getItem('access_token')
    fetch(url, { headers: token ? { Authorization: `Bearer ${token}` } : {} })
      .then((r) => r.blob())
      .then((blob) => {
        const a = document.createElement('a')
        a.href = URL.createObjectURL(blob)
        a.download = filename
        a.click()
        URL.revokeObjectURL(a.href)
      })
  }

  return (
    <div className="flex gap-2">
      <Button
        variant="outline"
        size="sm"
        onClick={() => download(jobsApi.downloadHtmlUrl(jobId), 'report.html')}
      >
        <FileCode className="mr-1.5 h-4 w-4" />
        HTML
      </Button>
      <Button
        variant="outline"
        size="sm"
        onClick={() => download(jobsApi.downloadDocxUrl(jobId), 'report.docx')}
      >
        <FileText className="mr-1.5 h-4 w-4" />
        DOCX
      </Button>
    </div>
  )
}
