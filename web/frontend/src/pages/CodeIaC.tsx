import { Code2 } from 'lucide-react'
import { ConnectionsPage } from '@/components/ConnectionsPage'

export function CodeIaC() {
  return (
    <ConnectionsPage
      connType="code"
      title="Code & IaC"
      description="Manage your code and infrastructure-as-code repositories"
      emptyDescription="Add a repository connection to start running code security audits."
      newConnectionPath="/connections/new?type=code"
      icon={Code2}
    />
  )
}
