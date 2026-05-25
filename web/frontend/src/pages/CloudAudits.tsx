import { Cloud } from 'lucide-react'
import { ConnectionsPage } from '@/components/ConnectionsPage'

export function CloudAudits() {
  return (
    <ConnectionsPage
      connType={['do', 'ssl', 'dns']}
      title="Cloud Audits"
      description="Manage your DigitalOcean, SSL/TLS and DNS security connections"
      emptyDescription="Add a connection to start running infrastructure, SSL/TLS or DNS security audits."
      newConnectionPath="/connections/new?type=do"
      icon={Cloud}
    />
  )
}
