import { Cloud } from 'lucide-react'
import { ConnectionsPage } from '@/components/ConnectionsPage'

export function CloudAudits() {
  return (
    <ConnectionsPage
      connType={['do', 'ssl', 'dns', 'aws']}
      title="Cloud Audits"
      description="Manage your DigitalOcean, AWS, SSL/TLS and DNS security connections"
      emptyDescription="Add a connection to start running infrastructure, SSL/TLS, DNS or AWS security audits."
      newConnectionPath="/connections/new?type=do"
      icon={Cloud}
    />
  )
}
